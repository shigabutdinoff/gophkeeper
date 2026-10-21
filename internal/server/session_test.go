package server

import (
	"context"
	"crypto/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (r *running) refresh(t *testing.T, token string) (int, string) {
	t.Helper()
	return r.post(t, "/auth/v1/token?grant_type=refresh_token", "app-key", map[string]string{"refresh_token": token})
}

func (r *running) sessions(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, connect(t, r.dsn).QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&n), "таблица сессий не прочитана")
	return n
}

func (r *running) exec(t *testing.T, sql string) {
	t.Helper()
	_, err := connect(t, r.dsn).Exec(context.Background(), sql)
	require.NoError(t, err, "запрос к базе не выполнен: %s", sql)
}

func expires(t *testing.T, r *running, token string) time.Time {
	t.Helper()
	claims, _, err := verify(t, r, token)
	require.NoError(t, err, "токен сервера не принят")
	return claims.ExpiresAt.Time
}

func TestRefresh(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, signed := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", signed)
	status, entered := r.login(t, email, password)
	require.Equal(t, http.StatusOK, status, "вход отклонён: %s", entered)
	for name, body := range map[string]string{"регистрация": signed, "вход": entered} {
		was := parse(t, body)
		require.NotEmpty(t, was.Refresh, "нет токена продления: %s", name)
		status, renewed := r.refresh(t, was.Refresh)
		assert.Equal(t, reference(t, "refresh_ok.txt"), answer(status, masked(t, renewed)), "ответ продления не равен эталону Supabase: %s", name)
		now := parse(t, renewed)
		assert.Equal(t, was.Refresh, now.Refresh, "продление сменило токен продления: %s", name)
		assert.Equal(t, was.User.ID, now.User.ID, "продление вернуло другого пользователя: %s", name)
		assert.False(t, expires(t, r, now.Access).Before(expires(t, r, was.Access)), "новый токен доступа истекает раньше прежнего: %s", name)
		assert.Equal(t, created(t, signed), created(t, renewed), "время создания пользователя разошлось: %s", name)
	}
	assert.Equal(t, created(t, signed), created(t, entered), "время создания пользователя во входе разошлось с регистрацией")
}

func created(t *testing.T, body string) string {
	t.Helper()
	_, rest, ok := strings.Cut(body, `"identities":`)
	require.True(t, ok, "в ответе нет пользователя: %s", body)
	_, at, ok := strings.Cut(rest, `],"created_at":"`)
	require.True(t, ok, "в ответе нет времени создания: %s", body)
	return at[:strings.IndexByte(at, '"')]
}

func TestConcurrentRefresh(t *testing.T) {
	r := start(t)
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	token := parse(t, body).Refresh
	got := make([]string, 8)
	statuses := make([]int, len(got))
	var wg sync.WaitGroup
	for i := range got {
		wg.Go(func() { statuses[i], got[i] = r.refresh(t, token) })
	}
	wg.Wait()
	for i := range got {
		assert.Equal(t, http.StatusOK, statuses[i], "одновременное продление отклонено: %s", got[i])
		assert.Equal(t, token, parse(t, got[i]).Refresh, "одновременное продление сменило токен продления")
	}
	status, body = r.refresh(t, token)
	assert.Equal(t, http.StatusOK, status, "продление после одновременных отклонено: %s", body)
}

func TestRefreshRefused(t *testing.T) {
	r := start(t)
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	for _, token := range []string{rand.Text(), "abcdefghijkl"} {
		status, body := r.refresh(t, token)
		assert.Equal(t, reference(t, "refresh_token_not_found.txt"), answer(status, body), "отказ незнакомому токену не равен эталону Supabase: %s", token)
	}
	for name, raw := range map[string]string{"негодный JSON": `не объект`, "пустой токен": `{"refresh_token":""}`, "без токена": `{}`} {
		status, body, _ := r.send(t, http.MethodPost, "/auth/v1/token?grant_type=refresh_token", "app-key", []byte(raw))
		assert.Equal(t, http.StatusBadRequest, status, "негодное продление получило не общий отказ: %s: %s", name, body)
		assert.Equal(t, "validation_failed", parse(t, body).Code, "негодное продление получило чужой код: %s", name)
	}
	assert.Equal(t, 1, r.sessions(t), "отказы изменили сессии")
}

func TestRefreshLostTable(t *testing.T) {
	r := start(t)
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	r.exec(t, "ALTER TABLE sessions RENAME TO sessions_off")
	status, refused := r.refresh(t, parse(t, body).Refresh)
	assert.Equal(t, http.StatusInternalServerError, status, "продление без таблицы не получило общий отказ: %s", refused)
	assert.Equal(t, "unexpected_failure", parse(t, refused).Code, "сбой базы показан клиенту чужой ошибкой")
	r.exec(t, "ALTER TABLE sessions_off RENAME TO sessions")
	status, _ = r.refresh(t, parse(t, body).Refresh)
	assert.Equal(t, http.StatusOK, status, "сервер не продлевает после сбоя базы")
}

func TestSignupWithoutSession(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился")
	r.exec(t, "ALTER TABLE sessions RENAME TO sessions_off")
	status, body := r.signup(t, email, password)
	assert.Equal(t, http.StatusInternalServerError, status, "регистрация без сессии не получила общий отказ: %s", body)
	assert.Equal(t, "unexpected_failure", parse(t, body).Code, "сбой сессии показан клиенту чужой ошибкой")
	assert.Equal(t, 1, r.rows(t, "WHERE email = $1", email), "учётная запись не осталась")
	r.exec(t, "ALTER TABLE sessions_off RENAME TO sessions")
	status, body = r.signup(t, email, password)
	assert.Equal(t, reference(t, "user_already_exists.txt"), answer(status, body), "повторная регистрация не равна эталону Supabase")
	status, body = r.login(t, email, password)
	assert.Equal(t, http.StatusOK, status, "вход после сбоя сессии отклонён: %s", body)
}

func TestSessionsSurviveRestart(t *testing.T) {
	dsn, key := database(t), tokenKey(t)
	r := startOn(t, dsn, key)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	entered := r.enter(t, email, password)
	assert.Equal(t, 2, r.sessions(t), "вход и регистрация создали не по сессии")
	assert.NotEqual(t, parse(t, body).Refresh, entered.Refresh, "две сессии получили один токен продления")
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	r = startOn(t, dsn, key)
	status, body = r.refresh(t, entered.Refresh)
	assert.Equal(t, http.StatusOK, status, "сессия не пережила перезапуск: %s", body)
}
