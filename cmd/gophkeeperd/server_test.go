package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	defer l.Close()
	return l.Addr().String()
}

func serverFiles(t *testing.T, database string) *http.Client {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	srv.Close()
	pair := srv.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	require.NoError(t, err, "ключ не закодирован")
	signing, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ подписи токенов не создан")
	token, err := x509.MarshalPKCS8PrivateKey(signing)
	require.NoError(t, err, "ключ подписи токенов не закодирован")
	dir := t.TempDir()
	for name, block := range map[string]*pem.Block{"cert.pem": {Type: "CERTIFICATE", Bytes: pair.Certificate[0]},
		"key.pem": {Type: "PRIVATE KEY", Bytes: key}, "token-s3cret.pem": {Type: "PRIVATE KEY", Bytes: token}} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(block), 0o600), "файл %s не записан", name)
	}
	for k, v := range map[string]string{"CONFIG": "", "GOPHKEEPER_ADDRESS": "", "GOPHKEEPER_CERT": filepath.Join(dir, "cert.pem"),
		"GOPHKEEPER_KEY": filepath.Join(dir, "key.pem"), "GOPHKEEPER_TOKEN_KEY": filepath.Join(dir, "token-s3cret.pem"),
		"GOPHKEEPER_APP_KEY": "app-key", "GOPHKEEPER_SERVER_KEY": "service-key", "GOPHKEEPER_DATABASE": database} {
		t.Setenv(k, v)
	}
	return srv.Client()
}

func serverEnv(t *testing.T) *http.Client {
	t.Helper()
	base := os.Getenv("GOPHKEEPER_TEST_DATABASE")
	if base == "" {
		t.Skip("не задана GOPHKEEPER_TEST_DATABASE")
	}
	conn, err := pgx.Connect(context.Background(), base)
	require.NoError(t, err, "база не открыта")
	t.Cleanup(func() { conn.Close(context.Background()) })
	schema := "t_" + strings.ToLower(rand.Text())
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err, "схема не создана")
	t.Cleanup(func() { conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(base)
	require.NoError(t, err, "адрес базы не разобран")
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return serverFiles(t, u.String())
}

func startServer(ctx context.Context, args ...string) <-chan error {
	var out, errOut strings.Builder
	root := newRoot(&out, &errOut)
	root.SetArgs(append([]string{"server"}, args...))
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()
	return done
}

func answers(t *testing.T, client *http.Client, addr string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://"+addr+"/auth/v1/health", nil)
	require.NoError(t, err, "запрос не собран")
	req.Header.Set("apikey", "app-key")
	assert.Eventually(t, func() bool {
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusNotFound && resp.TLS != nil
	}, 5*time.Second, 10*time.Millisecond, "сервер не отвечает по защищённому соединению на %s", addr)
}

func TestServer(t *testing.T) {
	client := serverEnv(t)
	file, flag, env := freeAddr(t), freeAddr(t), freeAddr(t)
	path := filepath.Join(t.TempDir(), "server.yaml")
	require.NoError(t, os.WriteFile(path, []byte("address: "+file+"\n"), 0o600), "файл настроек не записан")
	for _, c := range []struct {
		want string
		args []string
	}{{env, []string{"-c", path, "--address", flag}}, {flag, []string{"-c", path, "--address", flag}}, {file, []string{"-c", path}}} {
		if c.want == env {
			t.Setenv("GOPHKEEPER_ADDRESS", env)
		} else {
			t.Setenv("GOPHKEEPER_ADDRESS", "")
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := startServer(ctx, c.args...)
		answers(t, client, c.want)
		cancel()
		require.NoError(t, <-done, "сервер остановился с ошибкой")
	}
}

func TestServerBusy(t *testing.T) {
	serverEnv(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	defer l.Close()
	err = <-startServer(t.Context(), "--address", l.Addr().String())
	require.Error(t, err, "сервер запустился на занятом адресе")
	assert.ErrorContains(t, err, "адрес "+l.Addr().String()+" не открыт: ", "ошибка не называет адрес")
}

func TestServerStartRefused(t *testing.T) {
	serverFiles(t, "postgres://postgres:s3cret@"+freeAddr(t)+"/base?sslmode=disable")
	addr := freeAddr(t)
	err := <-startServer(t.Context(), "--address", addr)
	require.ErrorContains(t, err, "база данных недоступна: ", "сервер запустился без базы")
	assert.NotContains(t, err.Error(), "s3cret", "секрет из настроек попал в ошибку")
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err, "адрес остался занят после отказа запуска")
	l.Close()
	t.Setenv("GOPHKEEPER_TOKEN_KEY", filepath.Join(t.TempDir(), "token-s3cret.pem"))
	err = <-startServer(t.Context(), "--address", addr)
	require.ErrorContains(t, err, "ключ подписи токенов не прочитан: ", "сервер запустился без файла ключа подписи")
	assert.NotContains(t, err.Error(), "s3cret", "путь ключа подписи попал в ошибку")
}

func TestServerPoolSettingsRefused(t *testing.T) {
	serverEnv(t)
	u, err := url.Parse(os.Getenv("GOPHKEEPER_DATABASE"))
	require.NoError(t, err, "адрес базы не разобран")
	password, _ := u.User.Password()
	t.Setenv("GOPHKEEPER_DATABASE", u.String()+"&pool_max_conns=4")
	addr := freeAddr(t)
	err = <-startServer(t.Context(), "--address", addr)
	require.ErrorContains(t, err, "база данных недоступна: ", "сервер принял настройку пула прежнего хранилища")
	assert.NotContains(t, err.Error(), ":"+password+"@", "пароль базы попал в ошибку")
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err, "адрес остался занят после отказа запуска")
	l.Close()
}

func TestServerSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("сигнал себе в Windows не отправить")
	}
	client := serverEnv(t)
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			addr := freeAddr(t)
			done := startServer(context.Background(), "--address", addr)
			answers(t, client, addr)
			self, err := os.FindProcess(os.Getpid())
			require.NoError(t, err, "текущий процесс не найден")
			require.NoError(t, self.Signal(sig), "сигнал не отправлен")
			select {
			case err = <-done:
				require.NoError(t, err, "сервер остановился с ошибкой")
			case <-time.After(5 * time.Second):
				t.Fatal("сигнал не остановил сервер")
			}
		})
	}
}

func loadServer(t *testing.T, env string, args ...string) string {
	t.Helper()
	var s serverSettings
	fs := pflag.NewFlagSet("server", pflag.ContinueOnError)
	flags := addFlags(fs, &s)
	t.Setenv("CONFIG", "")
	for _, f := range flags {
		t.Setenv(f.EnvNames[0], strings.Repeat(env+"-"+f.Name, min(len(env), 1)))
	}
	require.NoError(t, fs.Parse(args), "флаги не разобраны")
	if err := load(fs, flags); err != nil {
		return "ошибка: " + err.Error()
	}
	return fmt.Sprintf("%+v", s)
}

func TestServerLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yaml")
	require.NoError(t, os.WriteFile(path, []byte("address: file-address\ncert: file-cert\nkey: file-key\ntoken_key: file-token-key\napp_key: file-app-key\nserver_key: file-server-key\ndatabase: file-database\n"), 0o600), "файл настроек не записан")
	flags := []string{"-c", path, "--address", "flag-address", "--cert", "flag-cert", "--key", "flag-key", "--token-key", "flag-token-key",
		"--app-key", "flag-app-key", "--server-key", "flag-server-key", "--database", "flag-database"}
	snaps.MatchSnapshot(t, []string{
		"file: " + loadServer(t, "", "-c", path),
		"flag: " + loadServer(t, "", flags...),
		"env: " + loadServer(t, "env", flags...),
		"none: " + loadServer(t, ""),
	})
}
