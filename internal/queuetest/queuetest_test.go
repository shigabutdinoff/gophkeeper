package queuetest

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const anna = "0199b5e4-0000-7000-8000-00000000a11a"

func publish(t *testing.T, q *Queue) uint64 {
	t.Helper()
	ack, err := JetStream(t, q.Client, q.As(t, anna)).Publish(t.Context(), "changes.records."+anna, []byte("изменение"))
	require.NoError(t, err, "клиент не отправил изменение")
	return ack.Sequence
}

func TestQueue(t *testing.T) {
	q := New(t)
	_, stream := q.Start(t, false)
	assert.Equal(t, uint64(1), publish(t, q), "изменение не первое в потоке")
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	assert.Equal(t, uint64(1), info.State.Msgs, "изменения нет в потоке")
	q.Server.Shutdown()
	q.Start(t, false)
	assert.Equal(t, uint64(1), publish(t, q), "после перезапуска клиент не отправил изменение")
}

func TestQueueRights(t *testing.T) {
	q := New(t)
	q.Start(t, false)
	for name, c := range map[string]struct {
		extra     nats.Option
		forbidden bool
	}{"client": {q.As(t, anna), true}, "admin": {nats.UserInfo(q.Admin.QueueUser, q.Admin.QueuePass), false}} {
		nc := JetStream(t, q.Client, c.extra, nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {})).Conn()
		_, err := nc.SubscribeSync("changes.>")
		require.NoError(t, err, "подписка %s не отправлена", name)
		require.NoError(t, nc.Flush(), "очередь не ответила %s", name)
		assert.Equal(t, c.forbidden, errors.Is(nc.LastError(), nats.ErrPermissionViolation), "права %s на чтение изменений не те", name)
	}
}
