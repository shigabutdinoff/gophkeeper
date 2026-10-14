package queuetest

import (
	"net/http"
	"testing"
	"time"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
)

// Run запускает очередь с потоком CHANGES в памяти без службы доступа и
// возвращает JetStream администратора и поток, при noAck поток не
// подтверждает изменения. Повторный запуск занимает тот же порт.
func (q *Queue) Run(t testing.TB, noAck bool) (jetstream.JetStream, jetstream.Stream) {
	t.Helper()
	q.Server = natstest.RunServer(q.opts)
	t.Cleanup(q.Server.Shutdown)
	js := JetStream(t, q.Admin)
	stream, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: "CHANGES", Subjects: []string{"changes.>"},
		Storage: jetstream.MemoryStorage, Duplicates: 2 * time.Minute, NoAck: noAck})
	require.NoError(t, err, "поток CHANGES не создан")
	return js, stream
}

// Start запускает очередь, как Run, и службу доступа для токенов Supabase.
func (q *Queue) Start(t testing.TB, noAck bool) (jetstream.JetStream, jetstream.Stream) {
	t.Helper()
	js, stream := q.Run(t, noAck)
	stop, err := access.Serve(t.Context(), js.Conn(), q.Seed, "", q.Supabase, http.DefaultClient, q.Server)
	require.NoError(t, err, "служба доступа не запущена")
	t.Cleanup(func() { _ = stop() })
	return js, stream
}
