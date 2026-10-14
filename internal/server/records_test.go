package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const listQuery = "?select=id,note,secret&order=id&limit=100"

func (r *running) records(t *testing.T, query, token string) (int, string) {
	t.Helper()
	h := http.Header{"Apikey": {"app-key"}}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	got := r.get(t, "/rest/v1/records"+query, h)
	return got.status, got.body
}

func (r *running) user(t *testing.T) granted {
	t.Helper()
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	return parse(t, body)
}

func (r *running) put(t *testing.T, owner, note string) string {
	t.Helper()
	id := uuid.New().String()
	_, err := connect(t, r.dsn).Exec(context.Background(), "INSERT INTO records (id, user_id, note, secret) VALUES ($1, $2, $3, 'c2VjcmV0')", id, owner, note)
	require.NoError(t, err, "запись не добавлена")
	return id
}

func (r *running) fill(t *testing.T, owner string, n int) {
	t.Helper()
	_, err := connect(t, r.dsn).Exec(context.Background(),
		"INSERT INTO records (id, user_id, secret) SELECT gen_random_uuid(), $1, 'x' FROM generate_series(1, $2)", owner, n)
	require.NoError(t, err, "записи не добавлены")
}

func list(t *testing.T, body string) []entry {
	t.Helper()
	var got []entry
	require.NoError(t, json.Unmarshal([]byte(body), &got), "ответ не массив записей: %s", body)
	return got
}

func TestRecordsPaged(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	r.fill(t, anna.User.ID, 101)
	var ids []string
	query := listQuery
	for {
		status, body := r.records(t, query, anna.Access)
		require.Equal(t, http.StatusOK, status, "список отклонён: %s", body)
		page := list(t, body)
		for _, e := range page {
			ids = append(ids, e.ID)
		}
		if len(page) < 100 {
			break
		}
		query = listQuery + "&id=gt." + ids[len(ids)-1]
	}
	rows, err := connect(t, r.dsn).Query(context.Background(), "SELECT id::text FROM records WHERE user_id = $1 ORDER BY id", anna.User.ID)
	require.NoError(t, err, "записи не прочитаны")
	var want []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id), "запись не прочитана")
		want = append(want, id)
	}
	assert.Len(t, want, 101, "в базе не все записи")
	assert.Equal(t, want, ids, "список страницами не равен записям владельца по возрастанию")
}

func TestRecordsOwn(t *testing.T) {
	r := start(t)
	anna, boris := r.user(t), r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	status, body := r.records(t, "?select=id,note,secret&id=eq."+id, anna.Access)
	assert.Equal(t, reference(t, "records_ok.txt"), answer(status, masked(t, body)), "своя запись не равна эталону Supabase")
	assert.Equal(t, id, list(t, body)[0].ID, "отдана не та запись")
	for name, c := range map[string][2]string{"чужая запись": {id, boris.Access}, "несуществующая запись": {uuid.New().String(), anna.Access}} {
		status, body = r.records(t, "?select=id,note,secret&id=eq."+c[0], c[1])
		assert.Equal(t, reference(t, "records_none.txt"), answer(status, body), "ответ не равен эталону Supabase: %s", name)
	}
	status, body = r.records(t, listQuery, boris.Access)
	assert.Equal(t, "200\n[]", answer(status, body), "список Бориса содержит записи Анны")
	empty := r.put(t, boris.User.ID, "")
	status, body = r.records(t, "?id=eq."+empty, boris.Access)
	assert.Equal(t, http.StatusOK, status, "запись с пустой пометкой не отдана")
	assert.Contains(t, body, `"note":""`, "пустая пометка пропала из ответа")
}

func TestRecordsRefused(t *testing.T) {
	dsn, key := database(t), tokenKey(t)
	r := startOn(t, dsn, key)
	anna := r.user(t)
	r.put(t, anna.User.ID, "почта")
	claims, _, err := verify(t, r, anna.Access)
	require.NoError(t, err, "токен сервера не принят")
	own, err := loadKey(key)
	require.NoError(t, err, "ключ сервера не прочитан")
	other, err := loadKey(tokenKey(t))
	require.NoError(t, err, "чужой ключ не прочитан")
	issuer := "https://" + r.addr + "/auth/v1"
	expired, err := own.sign(issuer, anna.User.ID, claims.ID, time.Now().Add(-time.Minute))
	require.NoError(t, err, "просроченный токен не подписан")
	forged, err := other.sign(issuer, anna.User.ID, claims.ID, time.Now().Add(lifetime))
	require.NoError(t, err, "токен чужим ключом не подписан")
	for name, token := range map[string]string{"без токена": "", "испорченный токен": spoil(anna.Access), "просроченный токен": expired, "чужой ключ": forged} {
		status, body := r.records(t, listQuery, token)
		assert.Equal(t, reference(t, "records_jwt_invalid.txt"), answer(status, body), "отказ не равен эталону Supabase: %s", name)
	}
	status, body, _ := r.send(t, http.MethodGet, "/rest/v1/records"+listQuery, "", nil)
	assert.Equal(t, http.StatusUnauthorized, status, "запрос без ключа проекта принят")
	assert.Contains(t, body, "No API key found in request", "отказ без ключа проекта изменился")
}

func TestRecordsFilters(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	id := uuid.New().String()
	for _, query := range []string{"?id=eq.abc", "?id=lt." + id, "?id=" + id, "?limit=-1", "?limit=x", "?limit="} {
		status, body := r.records(t, query, anna.Access)
		assert.Equal(t, http.StatusBadRequest, status, "негодный запрос принят: %s", query)
		assert.Equal(t, syntax, body, "отказ не в форме PostgREST: %s", query)
	}
	r.fill(t, anna.User.ID, 1500)
	status, body := r.records(t, "", anna.Access)
	require.Equal(t, http.StatusOK, status, "список без предела отклонён: %s", body)
	assert.Len(t, list(t, body), 1000, "ответ без предела не ограничен 1000 строками")
	status, body = r.records(t, "?limit=5000", anna.Access)
	require.Equal(t, http.StatusOK, status, "список с большим пределом отклонён: %s", body)
	assert.Len(t, list(t, body), 1000, "ответ с большим пределом не ограничен 1000 строками")
}

func TestRecordsUpgrade(t *testing.T) {
	dsn := database(t)
	_, err := migrated(t, dsn).UpTo(context.Background(), 2)
	require.NoError(t, err, "схема прежней версии не создана")
	email, password := newEmail(), rand.Text()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err, "хеш не создан")
	var owner string
	require.NoError(t, connect(t, dsn).QueryRow(context.Background(),
		"INSERT INTO accounts (email, encrypted_password) VALUES ($1, $2) RETURNING id::text", email, string(hash)).Scan(&owner), "учётная запись не добавлена")
	_, err = connect(t, dsn).Exec(context.Background(), "INSERT INTO sessions (account_id, refresh_token) VALUES ($1, 'old')", owner)
	require.NoError(t, err, "сессия не добавлена")
	r := startOn(t, dsn, tokenKey(t))
	status, body := r.refresh(t, "old")
	require.Equal(t, http.StatusOK, status, "сессия не пережила обновление схемы: %s", body)
	assert.Equal(t, []string{"id", "note", "secret", "updated_at", "user_id"}, columns(t, dsn, "records"), "столбцы таблицы записей не те")
	id := r.put(t, owner, "почта")
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	r = startOn(t, dsn, tokenKey(t))
	status, body = r.records(t, "?id=eq."+id, r.enter(t, email, password).Access)
	assert.Equal(t, reference(t, "records_ok.txt"), answer(status, masked(t, body)), "запись не пережила перезапуск")
}

func TestRecordsLogged(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "log"))
	require.NoError(t, err, "файл журнала не создан")
	r := start(t, func(s *Settings) { s.Log = out })
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	anna := parse(t, body)
	r.put(t, anna.User.ID, "почта")
	status, _ = r.records(t, listQuery, anna.Access)
	require.Equal(t, http.StatusOK, status, "список отклонён")
	r.records(t, "?limit=x", anna.Access)
	r.records(t, listQuery, spoil(anna.Access))
	r.login(t, email, rand.Text())
	r.exec(t, "ALTER TABLE sessions RENAME TO sessions_off")
	status, _ = r.refresh(t, anna.Refresh)
	require.Equal(t, http.StatusInternalServerError, status, "продление без таблицы не получило общий отказ")
	quiet, err := os.ReadFile(out.Name())
	require.NoError(t, err, "журнал не прочитан")
	assert.Empty(t, string(quiet), "журнал пишет не только внутренние ошибки чтения")
	r.exec(t, "ALTER TABLE records RENAME TO records_off")
	status, body = r.records(t, listQuery, anna.Access)
	assert.Equal(t, answer(http.StatusInternalServerError, internal), answer(status, body), "сбой чтения не получил общий отказ")
	logged, err := os.ReadFile(out.Name())
	require.NoError(t, err, "журнал не прочитан")
	lines := strings.Split(strings.TrimSuffix(string(logged), "\n"), "\n")
	require.Len(t, lines, 1, "в журнале не одна строка: %s", logged)
	assert.Regexp(t, `^time="\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d`, lines[0], "в строке журнала нет времени")
	for _, want := range []string{"level=error", `msg="внутренняя ошибка сервера"`, `relation \"records\" does not exist`, "path=/rest/v1/records"} {
		assert.Contains(t, lines[0], want, "в строке журнала нет причины или пути")
	}
	for _, secret := range []string{anna.Access, "c2VjcmV0", "select=", "limit"} {
		assert.NotContains(t, lines[0], secret, "в журнал попали токен, содержимое записи или строка запроса")
	}
}
