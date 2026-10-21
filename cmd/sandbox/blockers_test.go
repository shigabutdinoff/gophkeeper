package main

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/worker"
)

func TestStreamFreesHandled(t *testing.T) {
	t.Parallel()
	q := startQueue(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"user_id":"` + anna + `"}]`))
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx, q.admin, worker.Server{URL: srv.URL, Key: "ключ", Client: srv.Client(), Log: log.New(io.Discard, "", 0)})
	}()
	t.Cleanup(func() { cancel(); <-done })
	msg, _, err := record.Put(base64.RawURLEncoding.EncodeToString(make([]byte, 32)), anna, map[string]string{"login": "anna"}, "")
	require.NoError(t, err, "изменение не собрано")
	_, js := q.client(t, anna, nil)
	_, err = js.PublishMsg(t.Context(), msg)
	require.NoError(t, err, "изменение не принято очередью")
	_, err = js.Publish(t.Context(), record.Topic(anna), []byte("подделка"))
	require.NoError(t, err, "подделка не принята очередью")
	stream, err := q.admin.Stream(t.Context(), "CHANGES")
	require.NoError(t, err, "поток CHANGES недоступен")
	assert.Eventually(t, func() bool {
		info, ierr := stream.Info(t.Context())
		return ierr == nil && info.State.Msgs == 0
	}, 5*time.Second, 10*time.Millisecond, "обработанные изменения остались в потоке")
}

func TestStopIgnoresQueueAck(t *testing.T) {
	t.Parallel()
	q := startQueue(t)
	stopped := make(chan struct{}, 2)
	closeStop, err := listenStop(q.server, q.state, func() { stopped <- struct{}{} })
	require.NoError(t, err, "команда остановки не слушается")
	t.Cleanup(closeStop)
	device, _ := q.client(t, anna, nil)
	require.NoError(t, device.PublishRequest(record.Topic(anna), stopSubject, []byte("подмена")), "подмена не отправлена")
	require.NoError(t, device.Flush(), "очередь не ответила клиенту")
	select {
	case <-stopped:
		t.Fatal("подтверждение очереди остановило песочницу")
	case <-time.After(300 * time.Millisecond):
	}
	admin, err := sandboxUser(q.state, adminUser)
	require.NoError(t, err, "пароль администратора не прочитан")
	nc, _ := connect(t, q.url, q.ca, nil, admin, nats.CustomInboxPrefix("_INBOX.adm"))
	_, err = nc.Request(stopSubject, nil, 5*time.Second)
	require.NoError(t, err, "команда остановки не принята")
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("команда остановки не остановила песочницу")
	}
}
