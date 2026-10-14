package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func untrusting(t *testing.T) {
	t.Helper()
	setVar(t, &httpClient, &http.Client{Timeout: 2 * time.Second})
}

func certOf(t *testing.T, srv *signupServer) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600), "сертификат не записан")
	return path
}

func TestServerCert(t *testing.T) {
	keyring.MockInit()
	f := serveFile(t, 411, whole)
	f.settings.ServerCert = certOf(t, f.signupServer)
	writeSettings(t, f.settings)
	untrusting(t)
	before := f.requests.Load()
	login(t)
	assert.Greater(t, f.requests.Load(), before, "вход с файлом сертификата не дошёл до сервера")
	path := target(t)
	_, _, err := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
	require.NoError(t, err, "файл с файлом сертификата не получен")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "файл не записан")
	assert.Equal(t, f.plain, got, "файл не равен исходному")
	untrusting(t)
	_, _, err = execute("logout")
	require.NoError(t, err, "выход не прошёл")
	assert.Equal(t, "/auth/v1/logout", f.paths[len(f.paths)-1], "выход с файлом сертификата не дошёл до сервера")
}

func TestUntrustedServer(t *testing.T) {
	srv := startServer(t, ok)
	home := settingsFile(t, srv.URL, "ключ-приложения")
	untrusting(t)
	out, _, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.ErrorIs(t, err, account.ErrUntrusted, "вход прошёл на сервере с недоверенным сертификатом")
	assert.Empty(t, out, "при ошибке напечатан stdout")
	missing := filepath.Join(home, "missing.pem")
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", ServerCert: missing})
	_, _, err = executeIn(keyboard(t, true, typedPassword), "login")
	require.Error(t, err, "вход прошёл с негодным файлом сертификата")
	snaps.MatchSnapshot(t, strings.ReplaceAll(err.Error(), missing, "<файл>"))
	assert.Zero(t, srv.requests.Load(), "сервер получил запрос")
}

func TestUntrustedAdd(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	srv := loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	before := srv.requests.Load()
	untrusting(t)
	out, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.ErrorIs(t, err, account.ErrUntrusted, "запись принята на сервере с недоверенным сертификатом")
	assert.Empty(t, out, "при ошибке напечатано подтверждение")
	assert.Zero(t, queued(t, stream), "изменение ушло в очередь")
	assert.Equal(t, before, srv.requests.Load(), "сервер получил запрос")
}

func TestUnknownOperation(t *testing.T) {
	srv := startServer(t, http.NotFound)
	settingsFile(t, srv.URL, "ключ-приложения")
	_, _, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.EqualError(t, err, "сервер вернул ошибку, попробуйте позже")
}

func TestLogoutBadServerCert(t *testing.T) {
	keyring.MockInit()
	srv := startServer(t, ok)
	home := settingsFile(t, srv.URL, "ключ-приложения")
	login(t)
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", ServerCert: filepath.Join(home, "missing.pem")})
	_, _, err := execute("logout")
	require.NoError(t, err, "выход с негодным файлом сертификата вернул ошибку")
	_, err = keyring.Get("gophkeeper", "session")
	assert.ErrorIs(t, err, keyring.ErrNotFound, "вход не удалён из хранилища секретов")
}

func strangerCert(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ не создан")
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err, "сертификат не выпущен")
	path := filepath.Join(t.TempDir(), "stranger.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600), "сертификат не записан")
	return path
}

func logoutServer(t *testing.T) *signupServer {
	t.Helper()
	keyring.MockInit()
	return startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/logout" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ok(w, r)
	})
}

func TestLogoutAfterSettingsMovedToOwnServer(t *testing.T) {
	srv := logoutServer(t)
	settingsFile(t, srv.URL, "ключ-приложения")
	login(t)
	httpClient.CloseIdleConnections()
	writeSettings(t, config.Settings{Server: "https://127.0.0.1:54443", AppKey: "другой", ServerCert: strangerCert(t)})
	_, _, err := execute("logout")
	require.NoError(t, err, "выход не прошёл")
	assert.Equal(t, "/auth/v1/logout", srv.paths[len(srv.paths)-1], "выход не дошёл до сервера входа после смены настроек")
}

func TestLogoutAfterSettingsMovedFromOwnServer(t *testing.T) {
	srv := logoutServer(t)
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", ServerCert: certOf(t, srv)})
	untrusting(t)
	login(t)
	settingsFile(t, "https://example.invalid", "другой")
	untrusting(t)
	_, _, err := execute("logout")
	require.NoError(t, err, "выход не прошёл")
	assert.Equal(t, "/auth/v1/logout", srv.paths[len(srv.paths)-1], "выход не дошёл до сервера с собственным сертификатом после смены настроек")
}

func TestLogoutFollowsServerCertSetting(t *testing.T) {
	srv := logoutServer(t)
	old := certOf(t, srv)
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", ServerCert: old})
	untrusting(t)
	login(t)
	require.NoError(t, os.Remove(old), "прежний файл сертификата не удалён")
	writeSettings(t, config.Settings{Server: srv.URL + "/", AppKey: "ключ-приложения", ServerCert: certOf(t, srv)})
	untrusting(t)
	_, _, err := execute("logout")
	require.NoError(t, err, "выход не прошёл")
	assert.Equal(t, "/auth/v1/logout", srv.paths[len(srv.paths)-1], "выход не взял новый файл сертификата из настроек")
}

func TestLogoutNeedsServerCertSetting(t *testing.T) {
	srv := logoutServer(t)
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", ServerCert: certOf(t, srv)})
	untrusting(t)
	login(t)
	before := srv.requests.Load()
	settingsFile(t, srv.URL, "ключ-приложения")
	untrusting(t)
	_, _, err := execute("logout")
	require.NoError(t, err, "выход без настройки сертификата вернул ошибку")
	assert.Equal(t, before, srv.requests.Load(), "без настройки сертификата сервер получил запрос")
	_, err = keyring.Get("gophkeeper", "session")
	assert.ErrorIs(t, err, keyring.ErrNotFound, "вход не удалён из хранилища секретов")
}
