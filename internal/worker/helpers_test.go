package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const (
	anna  = "0199b5e4-0000-7000-8000-00000000a11a"
	boris = "0199b5e4-0000-7000-8000-0000000b0715"
)

func TestMain(m *testing.M) {
	maxDelay, heartbeat = 10*time.Millisecond, 100*time.Millisecond
	os.Exit(snapstest.CleanM{M: m}.Run())
}

func setVar[T any](t *testing.T, v *T, value T) {
	old := *v
	*v = value
	t.Cleanup(func() { *v = old })
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func change(t *testing.T, owner string, id, secret string) *nats.Msg {
	t.Helper()
	return noted(t, owner, id, "пометка", secret)
}

func noted(t *testing.T, owner string, id, note, secret string) *nats.Msg {
	t.Helper()
	return operation(t, owner, "put", id, note, secret)
}

func operation(t *testing.T, owner string, op, id, note, secret string) *nats.Msg {
	t.Helper()
	msg := nats.NewMsg(record.Topic(owner))
	var err error
	c := record.Change{Op: op, Record: id, Note: note}
	if secret != "" {
		c.Secret = []byte(secret)
	}
	msg.Data, err = json.Marshal(c)
	require.NoError(t, err, "изменение не собрано")
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	return msg
}

func send(t *testing.T, q *queuetest.Queue, msgs ...*nats.Msg) uint64 {
	t.Helper()
	js := queuetest.JetStream(t, q.Admin)
	var seq uint64
	for _, msg := range msgs {
		ack, err := js.PublishMsg(t.Context(), msg)
		require.NoError(t, err, "изменение не принято очередью")
		seq = ack.Sequence
	}
	return seq
}

func run(t *testing.T, js jetstream.JetStream, s Server) func() error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, js, s) }()
	stop := sync.OnceValue(func() error { cancel(); return <-done })
	t.Cleanup(func() { stop() })
	return stop
}

func consumerInfo(t *testing.T, js jetstream.JetStream) *jetstream.ConsumerInfo {
	t.Helper()
	c, err := js.Consumer(t.Context(), "CHANGES", durable)
	if err != nil {
		return nil
	}
	info, err := c.Info(t.Context())
	if err != nil {
		return nil
	}
	return info
}

func acked(t *testing.T, js jetstream.JetStream, seq uint64) {
	t.Helper()
	assert.Eventually(t, func() bool {
		info := consumerInfo(t, js)
		return info != nil && info.AckFloor.Stream >= seq && info.NumAckPending == 0 && info.NumPending == 0
	}, 5*time.Second, 10*time.Millisecond, "изменение %d не подтверждено", seq)
}
