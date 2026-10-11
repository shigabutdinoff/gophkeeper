package cli

import (
	"net/http"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const missing = "0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d"

func withQueue(t *testing.T, reply http.HandlerFunc) (*signupServer, jetstream.Stream) {
	t.Helper()
	return queueFor(t, time.Hour, reply)
}

func queueFor(t *testing.T, ttl time.Duration, reply http.HandlerFunc) (*signupServer, jetstream.Stream) {
	t.Helper()
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	srv := serveAs(t, q.Token(t, annaID, ttl), reply)
	queueSettings(t, srv.URL, q.Client)
	login(t)
	return srv, stream
}

func lastChange(t *testing.T, stream jetstream.Stream) record.Change {
	t.Helper()
	msg, err := stream.GetLastMsgForSubject(t.Context(), record.Topic(annaID))
	require.NoError(t, err, "изменения нет в очереди")
	c, err := record.Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "изменение не разобрано")
	return c
}

func queued(t *testing.T, stream jetstream.Stream) uint64 {
	t.Helper()
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	return info.State.Msgs
}

func secretOf(t *testing.T, c record.Change) map[string]string {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	fields, err := record.Open(data, c.Record, c.Secret)
	require.NoError(t, err, "секрет правки не открыт")
	return fields
}
