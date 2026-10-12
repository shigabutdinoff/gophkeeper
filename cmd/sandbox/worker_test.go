package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
			w.Write([]byte(`[{"user_id":"` + anna + `"}]`))
		}
	}))
	t.Cleanup(srv.Close)
	q.Supabase = srv.URL
	old := workerClient
	workerClient = srv.Client()
	t.Cleanup(func() { workerClient = old })
	t.Setenv(supabaseURL, srv.URL)
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	_, err := runWorker(t.Context(), q.Server, state, q.Seed)
	assert.EqualError(t, err, "не заданы SUPABASE_SERVICE_ROLE_KEY", "обработчик запущен без ключа администратора")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "ключ администратора")
	ctx, cancel := context.WithCancel(context.Background())
	_, err = runWorker(ctx, q.Server, state, "s3cret")
	require.Error(t, err, "обработчик запущен с неверным ключом выдачи доступа")
	done, err := runWorker(ctx, q.Server, state, q.Seed)
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
