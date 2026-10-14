package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func offline(t *testing.T) *auth {
	t.Helper()
	a, err := loadKey(tokenKey(t))
	require.NoError(t, err, "ключ подписи токенов не прочитан")
	a.db, err = gorm.Open(postgres.Open("postgres://postgres@127.0.0.1:1/base?sslmode=disable&connect_timeout=1"),
		&gorm.Config{Logger: logger.Discard, DisableAutomaticPing: true})
	require.NoError(t, err, "база не открыта")
	db, err := a.db.DB()
	require.NoError(t, err, "пул не получен")
	t.Cleanup(func() { db.Close() })
	return a
}

func TestOfflineRoutes(t *testing.T) {
	srv := httptest.NewServer(offline(t).routes(Settings{AppKey: "app-key", ServerKey: "service-key"}))
	t.Cleanup(srv.Close)
	creds := `{"email":"anna@example.com","password":"ключ входа"}`
	for name, c := range map[string]struct {
		method, path, key, body string
		status                  int
		code                    string
	}{
		"регистрация без базы":       {http.MethodPost, "/auth/v1/signup", "app-key", creds, http.StatusInternalServerError, "unexpected_failure"},
		"вход без базы":              {http.MethodPost, "/auth/v1/token?grant_type=password", "service-key", creds, http.StatusInternalServerError, "unexpected_failure"},
		"адрес не по форме":          {http.MethodPost, "/auth/v1/signup", "app-key", `{"email":"anna","password":"ключ"}`, http.StatusBadRequest, "validation_failed"},
		"ключ входа длиннее 72 байт": {http.MethodPost, "/auth/v1/signup", "app-key", `{"email":"anna@example.com","password":"` + strings.Repeat("я", 37) + `"}`, http.StatusBadRequest, "validation_failed"},
		"продление без базы":         {http.MethodPost, "/auth/v1/token?grant_type=refresh_token", "app-key", `{"refresh_token":"x"}`, http.StatusInternalServerError, "unexpected_failure"},
		"выход без базы":             {http.MethodPost, "/auth/v1/logout?scope=local", "app-key", "", http.StatusBadRequest, "validation_failed"},
		"без ключа проекта":          {http.MethodPost, "/auth/v1/signup", "", creds, http.StatusUnauthorized, ""},
		"незнакомый адрес":           {http.MethodPost, "/auth/v1/signup/", "app-key", creds, http.StatusNotFound, ""},
		"набор ключей":               {http.MethodGet, keysPath, "", "", http.StatusOK, ""},
	} {
		req, err := http.NewRequest(c.method, srv.URL+c.path, strings.NewReader(c.body))
		require.NoError(t, err, "запрос не собран")
		req.Header.Set("apikey", c.key)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err, "сервер не ответил: %s", name)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err, "ответ не прочитан: %s", name)
		assert.Equal(t, c.status, resp.StatusCode, "статус не тот: %s: %s", name, body)
		if c.code != "" {
			assert.Equal(t, c.code, parse(t, string(body)).Code, "код отказа не тот: %s", name)
		}
	}
}

func TestGrantMatchesSupabase(t *testing.T) {
	a := offline(t)
	acc := account{ID: uuid.New().String(), Email: "anna@example.com", Created: time.Now().Add(-time.Hour)}
	s := session{ID: uuid.New().String(), AccountID: acc.ID, RefreshToken: "refresh"}
	for name, weak := range map[string]json.RawMessage{"signup_ok.txt": nil, "token_ok.txt": json.RawMessage("null"), "refresh_ok.txt": nil} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "https://server.example/auth/v1/signup", nil)
		a.grant(c, acc, s, name != "signup_ok.txt", weak)
		assert.Equal(t, reference(t, name), answer(rec.Code, masked(t, rec.Body.String())), "ответ не равен эталону Supabase %s", name)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), "тип ответа не тот: %s", name)
		var claims jwt.RegisteredClaims
		_, _, err := jwt.NewParser().ParseUnverified(parse(t, rec.Body.String()).Access, &claims)
		require.NoError(t, err, "токен не разобран: %s", name)
		assert.Equal(t, "https://server.example/auth/v1", claims.Issuer, "издатель токена не адрес сервера: %s", name)
		assert.Equal(t, acc.ID, claims.Subject, "токен выдан другому пользователю: %s", name)
		assert.Equal(t, s.ID, claims.ID, "токен не несёт свою сессию: %s", name)
		assert.Equal(t, s.RefreshToken, parse(t, rec.Body.String()).Refresh, "токен продления не из сессии: %s", name)
		assert.Contains(t, rec.Body.String(), `Z"`, "время в ответе не в UTC: %s", name)
	}
}
