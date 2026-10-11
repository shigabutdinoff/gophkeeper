package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const anna = "0199b5e4-0000-7000-8000-00000000a11a"

func startWorker(ctx context.Context, t *testing.T) (*atomic.Int32, *strings.Builder, <-chan error) {
	t.Helper()
	q := queuetest.New(t)
	q.Run(t, false)
	var applied atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var row struct {
			Owner string `json:"user_id"`
		}
		switch {
		case r.URL.Path == "/auth/v1/.well-known/jwks.json":
			q.JWKS(w, r)
		case r.Header.Get("apikey") == "ключ-сервера" && r.Method == http.MethodPost && r.URL.Path == "/rest/v1/records":
			if json.NewDecoder(r.Body).Decode(&row) == nil && row.Owner == anna {
				applied.Add(1)
			}
			w.Write([]byte(`[{"user_id":"` + anna + `"}]`))
		}
	}))
	t.Cleanup(srv.Close)
	q.Supabase = srv.URL
	old := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = old })
	for k, v := range map[string]string{"CONFIG": "", "GOPHKEEPER_QUEUE": q.Admin.Queue, "GOPHKEEPER_QUEUE_CA": q.Admin.QueueCA,
		"GOPHKEEPER_QUEUE_USER": q.Admin.QueueUser, "GOPHKEEPER_QUEUE_PASSWORD": q.Admin.QueuePass,
		"GOPHKEEPER_QUEUE_CREDS_FILE": "", "GOPHKEEPER_SERVER": srv.URL, "GOPHKEEPER_SERVER_KEY": "ключ-сервера",
		"GOPHKEEPER_ACCESS_SEED": q.Seed} {
		t.Setenv(k, v)
	}
	msg, _, err := record.Put(base64.RawURLEncoding.EncodeToString(make([]byte, 32)), anna, map[string]string{"login": "anna", "password": "s3cret"}, "")
	require.NoError(t, err, "изменение не собрано")
	var out, errOut strings.Builder
	root := newRoot(&out, &errOut)
	root.SetArgs([]string{"worker"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()
	assert.ErrorIs(t, queue.Send(t.Context(), q.Client, msg), queue.ErrDenied, "очередь пустила устройство без входа")
	assert.Eventually(t, func() bool { return queue.Send(t.Context(), q.Client, msg, q.As(t, anna)) == nil },
		5*time.Second, 10*time.Millisecond, "обработчик не пустил вошедшее устройство в очередь")
	assert.Eventually(t, func() bool { return applied.Load() == 1 }, 5*time.Second, 10*time.Millisecond, "изменение не применено")
	return &applied, &out, done
}

func TestWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, out, done := startWorker(ctx, t)
	cancel()
	require.NoError(t, <-done, "обработчик остановился с ошибкой")
	assert.Empty(t, out.String(), "обработчик писал в stdout")
}

func TestWorkerSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("сигнал себе в Windows не отправить")
	}
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			_, _, done := startWorker(context.Background(), t)
			self, err := os.FindProcess(os.Getpid())
			require.NoError(t, err, "текущий процесс не найден")
			require.NoError(t, self.Signal(sig), "сигнал не отправлен")
			select {
			case err = <-done:
				require.NoError(t, err, "обработчик остановился с ошибкой")
			case <-time.After(5 * time.Second):
				t.Fatal("сигнал не остановил обработчик")
			}
		})
	}
}
