package server

import (
	"crypto/rand"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (r *running) logout(t *testing.T, query, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://"+r.addr+"/auth/v1/logout"+query, nil)
	require.NoError(t, err, "запрос не собран")
	req.Header.Set("apikey", "app-key")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client().Do(req)
	require.NoError(t, err, "сервер не ответил")
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "ответ не прочитан")
	return resp.StatusCode, string(got)
}

func (r *running) enter(t *testing.T, email, password string) granted {
	t.Helper()
	status, body := r.login(t, email, password)
	require.Equal(t, http.StatusOK, status, "вход отклонён: %s", body)
	return parse(t, body)
}

func spoil(token string) string {
	i, c := len(token)-10, "A"
	if token[i] == 'A' {
		c = "B"
	}
	return token[:i] + c + token[i+1:]
}

func TestLogout(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	laptop, desktop := r.enter(t, email, password), r.enter(t, email, password)
	status, body = r.refresh(t, laptop.Refresh)
	require.Equal(t, http.StatusOK, status, "продление отклонено: %s", body)
	before := r.sessions(t)
	status, body = r.logout(t, "?scope=local", parse(t, body).Access)
	assert.Equal(t, reference(t, "logout_ok.txt"), answer(status, body), "ответ выхода не равен эталону Supabase")
	assert.Equal(t, before-1, r.sessions(t), "выход завершил не одну сессию")
	status, body = r.refresh(t, laptop.Refresh)
	assert.Equal(t, reference(t, "refresh_token_not_found.txt"), answer(status, body), "продление завершённой сессии не равно эталону Supabase")
	status, body = r.refresh(t, desktop.Refresh)
	assert.Equal(t, http.StatusOK, status, "выход завершил сессию другого устройства: %s", body)
	status, _ = r.logout(t, "?scope=local", laptop.Access)
	assert.Equal(t, http.StatusNoContent, status, "повторный выход отклонён")
	assert.Equal(t, before-1, r.sessions(t), "повторный выход завершил ещё сессию")
}

func TestLogoutRefused(t *testing.T) {
	dsn, key := database(t), tokenKey(t)
	r := startOn(t, dsn, key)
	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	s := parse(t, body)
	claims, _, err := verify(t, r, s.Access)
	require.NoError(t, err, "токен сервера не принят")
	own, err := loadKey(key)
	require.NoError(t, err, "ключ сервера не прочитан")
	other, err := loadKey(tokenKey(t))
	require.NoError(t, err, "чужой ключ не прочитан")
	issuer := "https://" + r.addr + "/auth/v1"
	expired, err := own.sign(issuer, s.User.ID, claims.ID, time.Now().Add(-time.Minute))
	require.NoError(t, err, "просроченный токен не подписан")
	forged, err := other.sign(issuer, s.User.ID, claims.ID, time.Now().Add(lifetime))
	require.NoError(t, err, "токен чужим ключом не подписан")
	for name, c := range map[string][2]string{
		"без области": {"", s.Access}, "все устройства": {"?scope=global", s.Access}, "другие устройства": {"?scope=others", s.Access},
		"испорченный токен": {"?scope=local", spoil(s.Access)}, "просроченный токен": {"?scope=local", expired},
		"чужой ключ": {"?scope=local", forged}, "без токена": {"?scope=local", ""},
	} {
		status, body := r.logout(t, c[0], c[1])
		assert.Equal(t, http.StatusBadRequest, status, "выход не получил отказ: %s: %s", name, body)
		assert.Equal(t, "validation_failed", parse(t, body).Code, "выход получил чужой код: %s", name)
	}
	noSession, err := own.sign(issuer, s.User.ID, "", time.Now().Add(lifetime))
	require.NoError(t, err, "токен без сессии не подписан")
	status, body = r.logout(t, "?scope=local", noSession)
	assert.GreaterOrEqual(t, status, http.StatusBadRequest, "выход без сессии в токене принят: %s", body)
	assert.Equal(t, 1, r.sessions(t), "отказы выхода завершили сессию")
	status, body = r.refresh(t, s.Refresh)
	assert.Equal(t, http.StatusOK, status, "сессия не продлевается после отказов выхода: %s", body)
}

func TestLogoutDuringRefresh(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	kept, ended := parse(t, body), r.enter(t, email, password)
	out, statuses := 0, make([]int, 8)
	var wg sync.WaitGroup
	wg.Go(func() { out, _ = r.logout(t, "?scope=local", ended.Access) })
	for i := range statuses {
		wg.Go(func() { statuses[i], _ = r.refresh(t, kept.Refresh) })
	}
	wg.Wait()
	assert.Equal(t, http.StatusNoContent, out, "выход во время продлений отклонён")
	for _, s := range statuses {
		assert.Equal(t, http.StatusOK, s, "продление во время выхода отклонено")
	}
	assert.Equal(t, 1, r.sessions(t), "выход во время продлений завершил не ту сессию")
}
