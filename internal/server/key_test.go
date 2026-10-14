package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verify(t *testing.T, r *running, token string) (jwt.RegisteredClaims, *jwt.Token, error) {
	t.Helper()
	base := "https://" + r.addr + "/auth/v1"
	keys, err := keyfunc.NewDefaultOverrideCtx(t.Context(), []string{"https://" + r.addr + keysPath}, keyfunc.Override{Client: r.client()})
	require.NoError(t, err, "ключи сервера не получены")
	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, keys.Keyfunc, jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(base), jwt.WithAudience("authenticated"), jwt.WithExpirationRequired())
	return claims, parsed, err
}

func TestKeysPublished(t *testing.T) {
	r := start(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		status, _, header := r.send(t, method, keysPath, "", nil)
		assert.Equal(t, http.StatusOK, status, "ключи без ключа проекта не отданы: %s", method)
		assert.Equal(t, "application/json", header.Get("Content-Type"), "тип ответа с ключами не тот: %s", method)
	}
	body := r.get(t, keysPath, nil).body
	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &set), "ключи не JSON: %s", body)
	require.Len(t, set.Keys, 1, "в наборе не один ключ")
	assert.NotContains(t, set.Keys[0], "d", "закрытая часть ключа отдана")
	assert.Equal(t, "ES256", set.Keys[0]["alg"], "алгоритм ключа не ES256")
	assert.NotEmpty(t, set.Keys[0]["kid"], "у ключа нет идентификатора")
}

func TestTokenAccepted(t *testing.T) {
	r := start(t)
	a, err := loadKey(tokenKey(t))
	require.NoError(t, err, "чужой ключ не прочитан")
	owner := uuid.New().String()
	forged, err := a.sign("https://"+r.addr+"/auth/v1", owner, uuid.New().String(), time.Now().Add(lifetime))
	require.NoError(t, err, "токен чужим ключом не подписан")
	_, _, err = verify(t, r, forged)
	assert.Error(t, err, "токен с чужой подписью принят")

	status, body := r.signup(t, newEmail(), rand.Text())
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	claims, token, err := verify(t, r, parse(t, body).Access)
	require.NoError(t, err, "токен сервера не принят")
	assert.Equal(t, parse(t, body).User.ID, claims.Subject, "токен выдан другому пользователю")
	_, err = uuid.Parse(claims.Subject)
	assert.NoError(t, err, "пользователь в токене не UUID")
	assert.InDelta(t, time.Hour.Seconds(), time.Until(claims.ExpiresAt.Time).Seconds(), 60, "срок токена не час")
	assert.Contains(t, r.get(t, keysPath, nil).body, `"kid":"`+token.Header["kid"].(string)+`"`, "идентификатор ключа токена не из набора")
}

func TestTokenKeyFormats(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ не создан")
	sec1, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err, "ключ не закодирован")
	_, err = loadKey(keyFile(t, "EC PRIVATE KEY", sec1))
	assert.NoError(t, err, "ключ в SEC1 не прочитан")
	_, err = loadKey(tokenKey(t))
	assert.NoError(t, err, "ключ в PKCS8 не прочитан")
}

func TestTokenKeyRejected(t *testing.T) {
	cert, key, _ := certFiles(t)
	empty := filepath.Join(t.TempDir(), "empty.pem")
	require.NoError(t, os.WriteFile(empty, nil, 0o600), "пустой файл не записан")
	for name, path := range map[string]string{"нет файла": filepath.Join(t.TempDir(), "s3cret.pem"), "пустой файл": empty, "ключ RSA": key} {
		_, err := serveOnce(t, Settings{Cert: cert, Key: key, TokenKey: path, Database: "postgres://127.0.0.1:1/base"})
		require.ErrorContains(t, err, "ключ подписи токенов не прочитан: ", "сервер запустился с негодным ключом подписи: %s", name)
		assert.NotContains(t, err.Error(), "s3cret", "путь ключа подписи попал в ошибку: %s", name)
	}
}
