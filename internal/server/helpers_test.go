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

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

type running struct {
	addr                string
	dsn                 string
	cert, key, tokenKey string
	pool                *x509.CertPool
	q                   *queuetest.Queue
	js                  jetstream.JetStream
	limit               int64
	stop                context.CancelFunc
	done                chan struct{}
	err                 error
}

var serverLog io.Writer

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
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	r := &running{addr: "127.0.0.1:0", dsn: dsn, cert: cert, key: key, tokenKey: tokenKey, pool: pool, q: q, js: js, limit: 1 << 30}
	r.serve(t)
	return r
}

func (r *running) serve(t *testing.T) {
	t.Helper()
	l, err := net.Listen("tcp", r.addr)
	require.NoError(t, err, "порт не занят")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.addr, r.stop, r.done = l.Addr().String(), cancel, done
	go func() {
		defer close(done)
		r.err = Serve(ctx, l, Settings{Cert: r.cert, Key: r.key, AppKey: "app-key", ServerKey: "service-key", TokenKey: r.tokenKey, Database: r.dsn, Log: serverLog,
			Queue: r.js, FilesLimit: r.limit})
	}()
	t.Cleanup(func() { cancel(); <-done })
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
	return &http.Client{Timeout: 5 * time.Second, Transport: checked{&http.Transport{ForceAttemptHTTP2: len(protos) > 0,
		TLSClientConfig: &tls.Config{RootCAs: r.pool, NextProtos: protos}}}}
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
