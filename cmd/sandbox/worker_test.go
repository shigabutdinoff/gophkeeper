package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func TestSandboxWorker(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	state := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(state, adminUser), []byte(q.Admin.QueuePass), 0o600), "пароль не записан")
	require.NoError(t, os.WriteFile(filepath.Join(state, caFile), []byte(q.Admin.QueueCA), 0o600), "CA не записан")
	var applied atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var row struct {
			Owner string `json:"user_id"`
		}
		switch {
		case r.URL.Path == "/auth/v1/.well-known/jwks.json":
			q.JWKS(w, r)
		case r.Header.Get("apikey") == "ключ администратора" && r.Method == http.MethodPost && r.URL.Path == "/rest/v1/records":
			if json.NewDecoder(r.Body).Decode(&row) == nil && row.Owner == anna {
				applied.Add(1)
			}
			_, _ = w.Write([]byte(`[{"user_id":"` + anna + `"}]`))
		}
	}))
	t.Cleanup(srv.Close)
	q.Supabase = srv.URL
	t.Setenv(supabaseURL, srv.URL)
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	_, err := runWorker(t.Context(), q.Server, state, q.Seed, srv.Client())
	assert.EqualError(t, err, "не заданы SUPABASE_SERVICE_ROLE_KEY", "обработчик запущен без ключа администратора")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "ключ администратора")
	ctx, cancel := context.WithCancel(context.Background())
	_, err = runWorker(ctx, q.Server, state, "s3cret", srv.Client())
	require.Error(t, err, "обработчик запущен с неверным ключом выдачи доступа")
	done, err := runWorker(ctx, q.Server, state, q.Seed, srv.Client())
	require.NoError(t, err, "обработчик песочницы не запущен")
	msg, _, err := record.Put(base64.RawURLEncoding.EncodeToString(make([]byte, 32)), anna, map[string]string{"login": "anna", "password": "s3cret"}, "")
	require.NoError(t, err, "изменение не собрано")
	_, err = queuetest.JetStream(t, q.Client, q.As(t, anna)).PublishMsg(t.Context(), msg)
	require.NoError(t, err, "изменение не отправлено")
	assert.Eventually(t, func() bool { return applied.Load() == 1 }, 5*time.Second, 10*time.Millisecond, "изменение не применено")
	cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- done.Wait() }()
	select {
	case err = <-stopped:
		require.NoError(t, err, "обработчик песочницы остановился с ошибкой")
	case <-time.After(5 * time.Second):
		t.Fatal("обработчик песочницы не остановился")
	}
}

const annaRecord = "0199b5e4-0000-7000-8000-0000000000d1"

func TestSandboxWorkerOwnDeletes(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	var deleted atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/v1/.well-known/jwks.json":
			q.JWKS(w, r)
		case r.Method == http.MethodDelete && r.URL.Path == "/storage/v1/s3/files/"+anna+"/"+annaRecord:
			deleted.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/v1/records" && r.Header.Get("apikey") == "служебный ключ":
			deleted.Add(1)
			_, _ = w.Write([]byte(`[{"user_id":"` + anna + `"}]`))
		}
	}))
	t.Cleanup(srv.Close)
	q.Supabase = srv.URL
	state := t.TempDir()
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	for name, data := range map[string]string{adminUser: q.Admin.QueuePass, caFile: q.Admin.QueueCA + string(cert), serviceKeyFile: "служебный ключ", appKeyFile: "ключ"} {
		require.NoError(t, os.WriteFile(filepath.Join(state, name), []byte(data), 0o600), "файл песочницы не записан")
	}
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err, "порт сервера не разобран")
	t.Setenv(ownPortEnv, port)
	ctx, cancel := context.WithCancel(context.Background())
	done, err := runWorker(ctx, q.Server, state, q.Seed, &http.Client{})
	require.NoError(t, err, "обработчик песочницы своего сервера не запущен")
	msg, err := record.Delete(anna, annaRecord)
	require.NoError(t, err, "удаление не собрано")
	_, err = queuetest.JetStream(t, q.Client, q.As(t, anna)).PublishMsg(t.Context(), msg)
	require.NoError(t, err, "удаление не отправлено")
	assert.Eventually(t, func() bool { return deleted.Load() == 2 }, 5*time.Second, 10*time.Millisecond, "удаление не дошло до своего сервера")
	cancel()
	require.NoError(t, done.Wait(), "обработчик песочницы остановился с ошибкой")
}
