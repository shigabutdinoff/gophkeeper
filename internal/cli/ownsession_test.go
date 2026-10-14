package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

type place struct {
	home, record string
	session      []byte
}

func here(t *testing.T) place {
	t.Helper()
	p := place{home: os.Getenv("XDG_CONFIG_HOME")}
	p.record, _ = keyring.Get("gophkeeper", "session")
	p.session, _ = os.ReadFile(filepath.Join(p.home, "gophkeeper", "session.yaml"))
	return p
}

func (p place) enter(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", p.home)
	if p.session != nil {
		require.NoError(t, os.WriteFile(filepath.Join(p.home, "gophkeeper", "session.yaml"), p.session, 0o600), "файл входа не восстановлен")
	}
	require.NoError(t, keyring.Set("gophkeeper", "session", p.record), "запись входа не восстановлена")
}

func edit(t *testing.T, field, value string) {
	t.Helper()
	raw, err := keyring.Get("gophkeeper", "session")
	require.NoError(t, err, "записи входа нет")
	var r map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &r), "запись входа не JSON")
	r[field] = value
	changed, err := json.Marshal(r)
	require.NoError(t, err, "запись входа не собрана")
	require.NoError(t, keyring.Set("gophkeeper", "session", string(changed)), "запись входа не изменена")
}

func ownQueue(t *testing.T, s config.Settings) (config.Settings, jetstream.Stream) {
	t.Helper()
	q := queuetest.New(t)
	js, stream := q.Run(t, false)
	client := &http.Client{}
	require.NoError(t, account.Trust(client, s.ServerCert), "сертификату сервера нет доверия")
	stop, err := access.Serve(t.Context(), js.Conn(), q.Seed, "", s.Server, client, q.Server)
	require.NoError(t, err, "служба доступа не получила ключи своего сервера")
	t.Cleanup(func() { stop() })
	c := q.Client
	c.Server, c.AppKey, c.ServerCert = s.Server, s.AppKey, s.ServerCert
	return c, stream
}

func rename(t *testing.T, dsn, from, to string) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err, "база не открыта")
	defer conn.Close(context.Background())
	_, err = conn.Exec(context.Background(), "ALTER TABLE "+from+" RENAME TO "+to)
	require.NoError(t, err, "таблица %s не переименована", from)
}

func signup(t *testing.T) {
	t.Helper()
	_, errOut, err := executeIn(keyboard(t, true, typedPassword), "register")
	require.NoError(t, err, "регистрация не прошла: %s", errOut)
}

func addProbe(t *testing.T, name string) (string, error) {
	t.Helper()
	out, errOut, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", name)
	return out + errOut, err
}

func TestOwnServerLongBreak(t *testing.T) {
	s, _ := startOwnServer(t)
	c, stream := ownQueue(t, s)
	device(t, c)
	signup(t)
	edit(t, "access_token", "")
	said, err := addProbe(t, "anna")
	require.NoError(t, err, "запись после перерыва не принята: %s", said)
	assert.NotContains(t, said, "сначала войдите", "после перерыва клиент просит войти")
	assert.Equal(t, uint64(1), queued(t, stream), "в очереди не одно изменение")
}

func TestOwnServerLogoutOneDevice(t *testing.T) {
	s, _ := startOwnServer(t)
	c, stream := ownQueue(t, s)
	device(t, c)
	signup(t)
	laptop := here(t)
	writeSettings(t, c)
	login(t)
	desktop := here(t)

	laptop.enter(t)
	out, errOut, err := execute("logout")
	require.NoError(t, err, "выход не прошёл: %s", errOut)
	assert.Contains(t, out+errOut, "Выход выполнен.", "выход ничего не сообщил")
	_, err = addProbe(t, "laptop")
	assert.ErrorIs(t, err, account.ErrLoggedOut, "после выхода ноутбук пишет записи")

	desktop.enter(t)
	said, err := addProbe(t, "desktop")
	require.NoError(t, err, "выход на ноутбуке завершил вход на рабочем компьютере: %s", said)
	assert.Equal(t, uint64(1), queued(t, stream), "в очереди не одно изменение")

	laptop.enter(t)
	_, err = addProbe(t, "ended")
	assert.ErrorIs(t, err, account.ErrLoggedOut, "завершённая сессия продлилась")
	_, err = keyring.Get("gophkeeper", "session")
	assert.ErrorIs(t, err, keyring.ErrNotFound, "вход завершённой сессии не стёрт")
}

func TestOwnServerUnknownSession(t *testing.T) {
	s, _ := startOwnServer(t)
	device(t, s)
	signup(t)
	edit(t, "refresh_token", "abcdefghijkl")
	_, err := account.Unlock(t.Context(), httpClient, s, nil)
	assert.ErrorIs(t, err, account.ErrLoggedOut, "незнакомая сессия не распознана")
	_, err = keyring.Get("gophkeeper", "session")
	assert.ErrorIs(t, err, keyring.ErrNotFound, "вход незнакомой сессии не стёрт")
}

func TestOwnServerDatabaseDown(t *testing.T) {
	s, dsn := startOwnServer(t)
	device(t, s)
	signup(t)
	rename(t, dsn, "sessions", "sessions_off")
	secrets, err := account.Unlock(t.Context(), httpClient, s, nil)
	require.NoError(t, err, "сохранённый вход потерян при сбое базы")
	assert.ErrorIs(t, secrets.Down, account.ErrServer, "сбой базы показан не как ошибка сервера")
	assert.NotEmpty(t, secrets.Access, "сохранённый токен входа потерян")
	rename(t, dsn, "sessions_off", "sessions")
	_, err = account.Unlock(t.Context(), httpClient, s, nil)
	assert.NoError(t, err, "сбой базы стёр сохранённый вход")
}

func TestOwnServerSignupWithoutSession(t *testing.T) {
	s, dsn := startOwnServer(t)
	device(t, s)
	_, err := account.Unlock(t.Context(), httpClient, s, nil)
	require.Error(t, err, "вход есть до регистрации")
	rename(t, dsn, "sessions", "sessions_off")
	_, _, err = executeIn(keyboard(t, true, typedPassword), "register")
	assert.ErrorIs(t, err, account.ErrServer, "сбой сессии при регистрации не показан ошибкой сервера")
	rename(t, dsn, "sessions_off", "sessions")
	_, _, err = executeIn(keyboard(t, true, typedPassword), "register")
	assert.ErrorIs(t, err, account.ErrExists, "учётная запись не осталась после сбоя сессии")
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход после сбоя сессии не прошёл: %s", errOut)
	assert.Contains(t, out+errOut, "Вход выполнен.", "вход ничего не сообщил")
}
