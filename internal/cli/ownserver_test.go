package cli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/server"
)

func pemFile(t *testing.T, name, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600), "файл %s не записан", name)
	return path
}

func ownDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GOPHKEEPER_TEST_DATABASE")
	if base == "" {
		t.Skip("не задана GOPHKEEPER_TEST_DATABASE")
	}
	conn, err := pgx.Connect(context.Background(), base)
	require.NoError(t, err, "база не открыта")
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	schema := "t_" + strings.ToLower(rand.Text())
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err, "схема не создана")
	t.Cleanup(func() { _, _ = conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(base)
	require.NoError(t, err, "адрес базы не разобран")
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func startOwnServer(t *testing.T) (config.Settings, string) {
	t.Helper()
	dsn := ownDatabase(t)
	return startOwnOn(t, dsn), dsn
}

func startOwnOn(t *testing.T, dsn string) config.Settings {
	t.Helper()
	web := httptest.NewUnstartedServer(nil)
	web.StartTLS()
	web.Close()
	pair := web.TLS.Certificates[0]
	tlsKey, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	require.NoError(t, err, "ключ сервера не закодирован")
	signing, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ подписи токенов не создан")
	tokenKey, err := x509.MarshalPKCS8PrivateKey(signing)
	require.NoError(t, err, "ключ подписи токенов не закодирован")
	cert := pemFile(t, "cert.pem", "CERTIFICATE", pair.Certificate[0])
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	js, _ := queuetest.New(t).Run(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, l, server.Settings{Cert: cert, Key: pemFile(t, "key.pem", "PRIVATE KEY", tlsKey), AppKey: "ключ-приложения",
			ServerKey: "служебный-ключ", TokenKey: pemFile(t, "token.pem", "PRIVATE KEY", tokenKey), Database: dsn,
			Queue: js, FilesLimit: 1 << 30})
	}()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done, "свой сервер остановился с ошибкой")
	})
	setVar(t, &httpClient, &http.Client{Timeout: 5 * time.Second})
	resp, err := web.Client().Get("https://" + l.Addr().String() + "/auth/v1/.well-known/jwks.json")
	require.NoError(t, err, "свой сервер не запустился")
	_ = resp.Body.Close()
	return config.Settings{Server: "https://" + l.Addr().String(), AppKey: "ключ-приложения", ServerCert: cert}
}

func device(t *testing.T, s config.Settings) {
	t.Helper()
	keyring.MockInit()
	writeSettings(t, s)
}

func TestOwnServerSameAsSupabase(t *testing.T) {
	type said struct{ registered, entered string }
	var own, cloud said
	s, _ := startOwnServer(t)
	device(t, s)
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "register")
	require.NoError(t, err, "регистрация на своём сервере не прошла: %s", errOut)
	own.registered = out + errOut
	_, _, err = executeIn(keyboard(t, true, typedPassword), "register")
	assert.ErrorIs(t, err, account.ErrExists, "занятый адрес не распознан")

	device(t, s)
	out, errOut, err = executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход на своём сервере не прошёл: %s", errOut)
	own.entered = out + errOut
	_, _, err = executeIn(keyboard(t, true, "чужой-пароль"), "login")
	assert.ErrorIs(t, err, account.ErrCredentials, "неверный пароль не распознан")

	fake := startServer(t, ok)
	device(t, config.Settings{Server: fake.URL, AppKey: "ключ-приложения"})
	out, errOut, err = executeIn(keyboard(t, true, typedPassword), "register")
	require.NoError(t, err, "регистрация на подделке Supabase не прошла")
	cloud.registered = out + errOut
	device(t, config.Settings{Server: fake.URL, AppKey: "ключ-приложения"})
	out, errOut, err = executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход на подделке Supabase не прошёл")
	cloud.entered = out + errOut
	assert.Equal(t, cloud, own, "сообщения клиента на своём сервере не те, что с Supabase")
	assert.NotEmpty(t, own.registered, "регистрация ничего не сообщила")
}
