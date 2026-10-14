package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type running struct {
	addr string
	dsn  string
	pool *x509.CertPool
	stop context.CancelFunc
	done chan struct{}
	err  error
}

func set[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func certFiles(t *testing.T) (certFile, keyFile string, pool *x509.CertPool) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	srv.Close()
	pair := srv.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	require.NoError(t, err, "ключ не закодирован")
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0o600), "сертификат не записан")
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600), "ключ не записан")
	pool = x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return certFile, keyFile, pool
}

func start(t *testing.T) *running {
	t.Helper()
	return startOn(t, database(t), tokenKey(t))
}

func startOn(t *testing.T, dsn, tokenKey string) *running {
	t.Helper()
	cert, key, pool := certFiles(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{addr: l.Addr().String(), dsn: dsn, pool: pool, stop: cancel, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		r.err = Serve(ctx, l, Settings{Cert: cert, Key: key, AppKey: "app-key", ServerKey: "service-key", TokenKey: tokenKey, Database: dsn})
	}()
	t.Cleanup(func() { cancel(); <-r.done })
	return r
}

func (r *running) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("сервер не остановился")
	}
	return r.err
}

func (r *running) client(protos ...string) *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{ForceAttemptHTTP2: len(protos) > 0,
		TLSClientConfig: &tls.Config{RootCAs: r.pool, NextProtos: protos}}}
}

type reply struct {
	status      int
	kind, proto string
	secure      bool
	body        string
}

func (r *running) get(t *testing.T, path string, header http.Header) reply {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://"+r.addr+path, nil)
	require.NoError(t, err, "запрос не собран")
	req.Header = header
	resp, err := r.client().Do(req)
	require.NoError(t, err, "сервер не ответил")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "ответ не прочитан")
	return reply{resp.StatusCode, resp.Header.Get("Content-Type"), resp.Proto, resp.TLS != nil, string(body)}
}

func (r *running) dial(t *testing.T) *tls.Conn {
	t.Helper()
	conn, err := tls.Dial("tcp", r.addr, &tls.Config{RootCAs: r.pool})
	require.NoError(t, err, "защищённое соединение не установлено")
	t.Cleanup(func() { conn.Close() })
	return conn
}

func drip(conn net.Conn, head, slow string) {
	io.WriteString(conn, head)
	for i := range len(slow) {
		if _, err := io.WriteString(conn, slow[i:i+1]); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
