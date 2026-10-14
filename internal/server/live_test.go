package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type liveSupabase struct {
	server, app, service, user string
	client                     *http.Client
}

func (l liveSupabase) as(token string) liveSupabase {
	l.user = token
	return l
}

func (l liveSupabase) do(t *testing.T, method, path, bearer string, body any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err, "тело запроса не собрано")
	req, err := http.NewRequest(method, strings.TrimSuffix(l.server, "/")+"/"+path, bytes.NewReader(raw))
	require.NoError(t, err, "запрос не собран")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", l.app)
	if bearer != "" {
		req.Header.Set("apikey", bearer)
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if l.user != "" {
		req.Header.Set("Authorization", "Bearer "+l.user)
	}
	resp, err := l.client.Do(req)
	require.NoError(t, err, "Supabase не ответил")
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "ответ Supabase не прочитан")
	return resp.StatusCode, string(got)
}

func matchReference(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_SNAPS") == "true" {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644), "эталон Supabase не записан")
		return
	}
	assert.Equal(t, reference(t, name), got, "ответ Supabase разошёлся с эталоном %s", name)
}

func TestLiveSupabaseReferences(t *testing.T) {
	l := liveSupabase{os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_ANON_KEY"), os.Getenv("SUPABASE_SERVICE_ROLE_KEY"), "",
		&http.Client{Timeout: 30 * time.Second}}
	if os.Getenv("GOPHKEEPER_LIVE") == "" || l.server == "" || l.app == "" || l.service == "" {
		t.Skip("живой Supabase не задан")
	}
	email := "anna+gop21-" + time.Now().Format("20060102150405.000000") + "@example.com"
	creds := map[string]string{"email": email, "password": rand.Text()}
	status, created := l.do(t, http.MethodPost, "auth/v1/signup", "", creds)
	user := parse(t, created)
	require.NotEmpty(t, user.User.ID, "регистрация не вернула пользователя, ответ %d: %s", status, created)
	t.Cleanup(func() {
		gone, body := l.do(t, http.MethodDelete, "auth/v1/admin/users/"+user.User.ID, l.service, nil)
		assert.Equal(t, http.StatusOK, gone, "учётная запись %s не удалена: %s", email, body)
	})
	require.NotEmpty(t, user.Access, "регистрация не выдала токен, подтверждение email включено")
	matchReference(t, "signup_ok.txt", answer(status, masked(t, created)))
	status, taken := l.do(t, http.MethodPost, "auth/v1/signup", "", creds)
	matchReference(t, "user_already_exists.txt", answer(status, taken))
	status, entered := l.do(t, http.MethodPost, "auth/v1/token?grant_type=password", "", creds)
	matchReference(t, "token_ok.txt", answer(status, masked(t, entered)))
	status, refused := l.do(t, http.MethodPost, "auth/v1/token?grant_type=password", "", map[string]string{"email": email, "password": rand.Text()})
	matchReference(t, "invalid_credentials.txt", answer(status, refused))

	status, renewed := l.do(t, http.MethodPost, "auth/v1/token?grant_type=refresh_token", "", map[string]string{"refresh_token": parse(t, entered).Refresh})
	matchReference(t, "refresh_ok.txt", answer(status, masked(t, renewed)))
	status, out := l.as(parse(t, renewed).Access).do(t, http.MethodPost, "auth/v1/logout?scope=local", "", struct{}{})
	matchReference(t, "logout_ok.txt", answer(status, out))
	status, ended := l.do(t, http.MethodPost, "auth/v1/token?grant_type=refresh_token", "", map[string]string{"refresh_token": parse(t, renewed).Refresh})
	matchReference(t, "refresh_token_not_found.txt", answer(status, ended))
	status, unknown := l.do(t, http.MethodPost, "auth/v1/token?grant_type=refresh_token", "", map[string]string{"refresh_token": "abcdefghijkl"})
	assert.Equal(t, answer(status, ended), answer(status, unknown), "отказ незнакомому токену продления не равен отказу завершённой сессии")
	liveRecords(t, l, creds)
}

func liveRecords(t *testing.T, l liveSupabase, creds map[string]string) {
	t.Helper()
	status, body := l.do(t, http.MethodPost, "auth/v1/token?grant_type=password", "", creds)
	require.Equal(t, http.StatusOK, status, "вход отклонён: %s", body)
	s := parse(t, body)
	id := uuid.New().String()
	status, body = l.do(t, http.MethodPost, "rest/v1/records", l.service,
		map[string]string{"id": id, "user_id": s.User.ID, "note": "почта", "secret": "c2VjcmV0"})
	require.Equal(t, http.StatusCreated, status, "запись не добавлена: %s", body)
	status, body = l.as(s.Access).do(t, http.MethodGet, "rest/v1/records?select=id,note,secret&id=eq."+id, "", nil)
	matchReference(t, "records_ok.txt", answer(status, masked(t, body)))
	status, body = l.as(s.Access).do(t, http.MethodGet, "rest/v1/records?select=id,note,secret&id=eq."+uuid.New().String(), "", nil)
	matchReference(t, "records_none.txt", answer(status, body))
	status, body = l.as(spoil(s.Access)).do(t, http.MethodGet, "rest/v1/records?select=id,note,secret&order=id&limit=100", "", nil)
	matchReference(t, "records_jwt_invalid.txt", answer(status, body))
}
