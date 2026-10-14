package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

const rawGet = "GET / HTTP/1.1\r\nHost: localhost\r\napikey: app-key\r\n"

func appKey() http.Header {
	return http.Header{"Apikey": {"app-key"}}
}

func TestServe(t *testing.T) {
	var reached atomic.Int32
	r := start(t, func(s *Settings) {
		s.operations = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached.Add(1)
			http.NotFound(w, nil)
		})
	})
	got := r.get(t, "/auth/v1/health", appKey())
	assert.Equal(t, http.StatusNotFound, got.status, "сервер не ответил на запрос с ключом проекта")
	assert.True(t, got.secure, "соединение не защищено")
	assert.Equal(t, "HTTP/1.1", got.proto, "сервер ответил не по HTTP/1.1")

	r.pool = x509.NewCertPool()
	resp, err := r.client().Get("https://" + r.addr)
	if err == nil {
		_ = resp.Body.Close()
	}
	assert.ErrorContains(t, err, "certificate", "клиент без доверия сертификату получил ответ")

	plain, err := http.Get("http://" + r.addr)
	require.NoError(t, err, "сервер не ответил на незащищённый запрос")
	_ = plain.Body.Close()
	assert.Equal(t, http.StatusBadRequest, plain.StatusCode, "незащищённый запрос принят")
	assert.EqualValues(t, 1, reached.Load(), "до обработчика дошёл лишний запрос")
}

func TestProtocols(t *testing.T) {
	r := start(t)
	_, err := tls.Dial("tcp", r.addr, &tls.Config{RootCAs: r.pool, NextProtos: []string{"h2"}})
	assert.Error(t, err, "клиент только с HTTP/2 подключился")
	resp, err := r.client("h2", "http/1.1").Get("https://" + r.addr)
	require.NoError(t, err, "клиент с двумя протоколами не подключился")
	_ = resp.Body.Close()
	assert.Equal(t, "HTTP/1.1", resp.Proto, "сервер выбрал не HTTP/1.1")
}

func TestNoCert(t *testing.T) {
	cert, key, _ := certFiles(t)
	for _, s := range []Settings{{Key: key}, {Cert: cert}, {}} {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err, "порт не занят")
		assert.ErrorIs(t, Serve(t.Context(), l, s), ErrNoCert, "сервер запустился без сертификата или ключа")
		_ = l.Close()
	}
	assert.EqualError(t, ErrNoCert, "нужны сертификат и ключ сервера")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	defer func() { _ = l.Close() }()
	missing := filepath.Join(t.TempDir(), "cert.pem")
	err = Serve(t.Context(), l, Settings{Cert: missing, Key: key})
	assert.ErrorIs(t, err, fs.ErrNotExist, "сервер запустился без файла сертификата")
	assert.ErrorContains(t, err, "сертификат и ключ сервера не прочитаны: ")
}

func TestSlowClients(t *testing.T) {
	r := start(t, func(s *Settings) { s.readTimeout = time.Second })
	raw, err := net.Dial("tcp", r.addr)
	require.NoError(t, err, "соединение не открыто")
	defer func() { _ = raw.Close() }()
	conns := map[string]net.Conn{"молчит до TLS": raw, "молчит после TLS": r.dial(t), "заголовки по байту": r.dial(t),
		"тело по байту": r.dial(t), "простаивает после ответа": r.dial(t)}
	go drip(conns["заголовки по байту"], "", rawGet+strings.Repeat("X-Pad: 1\r\n", 5))
	go drip(conns["тело по байту"], rawGet+"Content-Length: 50\r\n\r\n", strings.Repeat("x", 50))
	go drip(conns["простаивает после ответа"], rawGet+"\r\n", "")
	began := time.Now()
	var wg sync.WaitGroup
	for name, conn := range conns {
		wg.Go(func() {
			_ = conn.SetReadDeadline(began.Add(5 * time.Second))
			_, _ = io.Copy(io.Discard, conn)
			assert.InDelta(t, time.Second, time.Since(began), float64(900*time.Millisecond), "соединение не закрыто в срок: %s", name)
		})
	}
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, http.StatusNotFound, r.get(t, "/", appKey()).status, "другой клиент не получил ответ")
	wg.Wait()
}

func TestGuard(t *testing.T) {
	r := start(t)
	var got []string
	for _, h := range []http.Header{{}, {"Apikey": {""}}, {"Apikey": {"wrong"}}} {
		resp := r.get(t, "/rest/v1/records", h)
		got = append(got, fmt.Sprintf("%d %s %s", resp.status, resp.kind, resp.body))
	}
	snaps.MatchSnapshot(t, got)
	for _, key := range []string{"app-key", "service-key"} {
		for _, path := range []string{"/", "/auth/v1/health"} {
			assert.Equal(t, http.StatusNotFound, r.get(t, path, http.Header{"Apikey": {key}}).status, "ключ проекта не принят: %s %s", key, path)
		}
		got := r.get(t, "/rest/v1/records", http.Header{"Apikey": {key}})
		assert.Equal(t, reference(t, "records_jwt_invalid.txt"), answer(got.status, got.body), "чтение без токена не получило отказ Supabase: %s", key)
	}
}

func TestGuardEmptyKeys(t *testing.T) {
	for _, key := range []string{"", "x"} {
		w := httptest.NewRecorder()
		guard(Settings{}, http.NotFoundHandler()).ServeHTTP(w, &http.Request{Header: http.Header{"Apikey": {key}}})
		assert.Equal(t, http.StatusUnauthorized, w.Code, "сервер без ключей проекта пропустил запрос")
	}
}

func TestShutdown(t *testing.T) {
	began, release := make(chan struct{}), make(chan struct{})
	r := start(t, func(s *Settings) {
		s.operations = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(began)
			<-release
			w.WriteHeader(http.StatusNoContent)
		})
	})
	status := make(chan int, 1)
	go func() { status <- r.get(t, "/", appKey()).status }()
	<-began
	r.stop()
	assert.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", r.addr)
		if err == nil {
			_ = conn.Close()
		}
		return err != nil
	}, 5*time.Second, 10*time.Millisecond, "сервер принимает соединения после остановки")
	select {
	case <-r.done:
		t.Fatal("сервер завершился до ответа на начатый запрос")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	assert.Equal(t, http.StatusNoContent, <-status, "начатый запрос не получил ответ")
	assert.NoError(t, r.wait(t), "сервер остановился с ошибкой")
}

func TestShutdownGrace(t *testing.T) {
	began, hang := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(hang) })
	r := start(t, func(s *Settings) {
		s.grace = 300 * time.Millisecond
		s.operations = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			close(began)
			<-hang
		})
	})
	go drip(r.dial(t), rawGet+"\r\n", "")
	<-began
	stopped := time.Now()
	r.stop()
	assert.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	assert.InDelta(t, 300*time.Millisecond, time.Since(stopped), float64(250*time.Millisecond), "сервер ждал не предел остановки")
}

func TestStoppedBeforeServe(t *testing.T) {
	cert, key, _ := certFiles(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт не занят")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	js, _ := queuetest.New(t).Run(t, false)
	assert.NoError(t, Serve(ctx, l, Settings{Cert: cert, Key: key, TokenKey: tokenKey(t), Database: database(t), Queue: js, FilesLimit: 1 << 20}),
		"сервер остановился с ошибкой")
	l, err = net.Listen("tcp", l.Addr().String())
	require.NoError(t, err, "порт остался занят")
	_ = l.Close()
}

func TestNoQueue(t *testing.T) {
	cert, key, _ := certFiles(t)
	dsn, signing := database(t), tokenKey(t)
	js, _ := queuetest.New(t).Run(t, false)
	for name, s := range map[string]Settings{"без очереди": {FilesLimit: 1 << 20}, "без предела": {Queue: js}, "предел меньше нуля": {Queue: js, FilesLimit: -1}} {
		s.Cert, s.Key, s.TokenKey, s.Database = cert, key, signing, dsn
		addr, err := serveOnce(t, s)
		assert.ErrorIs(t, err, ErrNoQueue, "сервер запустился: %s", name)
		l, err := net.Listen("tcp", addr.String())
		require.NoError(t, err, "порт остался занят: %s", name)
		_ = l.Close()
	}
	assert.EqualError(t, ErrNoQueue, "нужны подключение к очереди и предел корзины файлов")
}

func TestFilesBucket(t *testing.T) {
	r := start(t)
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился")
	names, info := filesBucket(t, r.js)
	assert.Equal(t, []string{".keep"}, names, "в новой корзине файлов не одна метка")
	assert.EqualValues(t, 1<<30, info.Config.MaxBytes, "предел корзины файлов не из настроек")
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	r.limit = 2 << 20
	r.serve(t)
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился снова")
	names, info = filesBucket(t, r.js)
	assert.Equal(t, []string{".keep"}, names, "повторный запуск продублировал метку")
	assert.EqualValues(t, 2<<20, info.Config.MaxBytes, "предел корзины файлов не обновлён")
}
