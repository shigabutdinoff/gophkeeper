package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const keysPath = "/auth/v1/.well-known/jwks.json"

func columns(t *testing.T, dsn, table string) []string {
	t.Helper()
	rows, err := connect(t, dsn).Query(context.Background(),
		"SELECT column_name::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1 ORDER BY 1", table)
	require.NoError(t, err, "столбцы не прочитаны")
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name), "столбец не прочитан")
		names = append(names, name)
	}
	return names
}

func serveOnce(t *testing.T, s Settings) (net.Addr, error) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	defer l.Close()
	done := make(chan error, 1)
	go func() { done <- Serve(t.Context(), l, s) }()
	select {
	case err = <-done:
		return l.Addr(), err
	case <-time.After(5 * time.Second):
		t.Fatal("сервер не отказался от запуска")
		return nil, nil
	}
}

func TestSchemaCreated(t *testing.T) {
	dsn := database(t)
	assert.Empty(t, columns(t, dsn, "accounts"), "таблица есть до запуска сервера")
	r := startOn(t, dsn, tokenKey(t))
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился")
	assert.Equal(t, []string{"created_at", "email", "encrypted_password", "id"}, columns(t, dsn, "accounts"), "столбцы таблицы не те")
	assert.Equal(t, []string{"account_id", "id", "refresh_token"}, columns(t, dsn, "sessions"), "столбцы таблицы сессий не те")
}

func TestAccountsSurviveRestart(t *testing.T) {
	dsn, key := database(t), tokenKey(t)
	r := startOn(t, dsn, key)
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился")
	_, err := connect(t, dsn).Exec(context.Background(), "INSERT INTO accounts (email, encrypted_password) VALUES ('anna@example.com', 'hash')")
	require.NoError(t, err, "учётная запись не добавлена")
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	r = startOn(t, dsn, key)
	assert.Equal(t, 1, r.rows(t, ""), "учётная запись не пережила перезапуск")
}

func TestExistingTableKept(t *testing.T) {
	dsn := database(t)
	_, err := connect(t, dsn).Exec(context.Background(), schemaSQL+"; INSERT INTO accounts (email, encrypted_password) VALUES ('anna@example.com', 'hash')")
	require.NoError(t, err, "таблица не подготовлена")
	r := startOn(t, dsn, tokenKey(t))
	got := r.get(t, "/rest/v1/records", http.Header{"Apikey": {"app-key"}})
	assert.Equal(t, http.StatusNotFound, got.status, "сервер не запустился на базе с готовой таблицей")
	assert.Equal(t, 1, r.rows(t, ""), "строки готовой таблицы изменились")
}

func TestNoDatabase(t *testing.T) {
	cert, key, _ := certFiles(t)
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	port := strconv.Itoa(closed.Addr().(*net.TCPAddr).Port)
	closed.Close()
	s := Settings{Cert: cert, Key: key, AppKey: "app-key", ServerKey: "service-key", TokenKey: tokenKey(t),
		Database: "postgres://postgres:s3cret@127.0.0.1:" + port + "/base?sslmode=disable"}
	addr, err := serveOnce(t, s)
	require.ErrorContains(t, err, "база данных недоступна: ", "сервер не сообщил о недоступной базе")
	assert.NotContains(t, err.Error(), "s3cret", "пароль базы попал в ошибку")
	l, err := net.Listen("tcp", addr.String())
	require.NoError(t, err, "порт остался занят")
	l.Close()
}

func TestPoolSettingsRefused(t *testing.T) {
	dsn := database(t)
	cert, key, _ := certFiles(t)
	s := Settings{Cert: cert, Key: key, AppKey: "app-key", ServerKey: "service-key", TokenKey: tokenKey(t), Database: dsn + "&pool_max_conns=4"}
	addr, err := serveOnce(t, s)
	require.ErrorContains(t, err, "база данных недоступна: ", "сервер принял настройку пула прежнего хранилища")
	assert.NotContains(t, err.Error(), ":test@", "пароль базы попал в ошибку")
	l, err := net.Listen("tcp", addr.String())
	require.NoError(t, err, "порт остался занят")
	l.Close()
	r := startOn(t, dsn, tokenKey(t))
	assert.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился без настройки пула")
}

func TestDatabaseAddressRejected(t *testing.T) {
	cert, key, _ := certFiles(t)
	s := Settings{Cert: cert, Key: key, TokenKey: tokenKey(t), Database: "postgres://postgres:s3cret@127.0.0.1:порт/base"}
	_, err := serveOnce(t, s)
	require.ErrorContains(t, err, "адрес базы данных не разобран: ", "сервер не сообщил о негодном адресе базы")
	assert.NotContains(t, err.Error(), "s3cret", "пароль базы попал в ошибку")
}

func migrated(t *testing.T, dsn string) *goose.Provider {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err, "база не открыта")
	t.Cleanup(func() { db.Close() })
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	require.NoError(t, err, "миграции не прочитаны")
	return p
}

func TestUpgradeKeepsAccounts(t *testing.T) {
	dsn := database(t)
	_, err := migrated(t, dsn).UpTo(context.Background(), 1)
	require.NoError(t, err, "схема прежней версии не создана")
	email, password := newEmail(), rand.Text()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err, "хеш не создан")
	_, err = connect(t, dsn).Exec(context.Background(), "INSERT INTO accounts (email, encrypted_password) VALUES ($1, $2)", email, string(hash))
	require.NoError(t, err, "учётная запись не добавлена")
	r := startOn(t, dsn, tokenKey(t))
	status, body := r.login(t, email, password)
	assert.Equal(t, reference(t, "token_ok.txt"), answer(status, masked(t, body)), "вход после обновления не равен эталону Supabase")
	assert.Equal(t, []string{"account_id", "id", "refresh_token"}, columns(t, dsn, "sessions"), "столбцы таблицы сессий не те")
	status, body = r.signup(t, email, password)
	assert.Equal(t, reference(t, "user_already_exists.txt"), answer(status, body), "повторная регистрация после обновления не равна эталону")
	status, body = r.login(t, email, rand.Text())
	assert.Equal(t, reference(t, "invalid_credentials.txt"), answer(status, body), "неверный пароль после обновления не равен эталону")
}

func TestSessionsDown(t *testing.T) {
	dsn := database(t)
	r := startOn(t, dsn, tokenKey(t))
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	_, err := migrated(t, dsn).Down(context.Background())
	require.NoError(t, err, "миграция сессий не откатилась")
	assert.Empty(t, columns(t, dsn, "sessions"), "таблица сессий осталась после отката")
	assert.Equal(t, 1, r.rows(t, ""), "откат тронул учётные записи")
}
