package queue

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func count(t *testing.T, q *queuetest.Queue) uint64 {
	t.Helper()
	stream, err := queuetest.JetStream(t, q.Admin).Stream(t.Context(), "CHANGES")
	require.NoError(t, err, "поток не найден")
	return stream.CachedInfo().State.Msgs
}

const anna = "0199b5e4-0000-7000-8000-00000000a11a"

func change(id string) *nats.Msg {
	msg := nats.NewMsg("changes.records." + anna)
	msg.Data = []byte("изменение")
	msg.Header.Set(nats.MsgIdHdr, id)
	return msg
}

func TestConnectExtra(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	nc, err := Connect(q.Admin, nats.Name("обработчик"))
	require.NoError(t, err, "администратор не подключился")
	defer nc.Close()
	require.NoError(t, nc.Flush(), "очередь не ответила")
	assert.Equal(t, "обработчик", nc.Opts.Name, "дополнительный параметр не применён")
}

func TestSendOnce(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	msg := change("изменение-1")
	require.NoError(t, Send(t.Context(), q.Client, msg, q.As(t, anna)), "изменение не отправлено")
	require.NoError(t, Send(t.Context(), q.Client, msg, q.As(t, anna)), "повтор изменения не отправлен")
	assert.Equal(t, uint64(1), count(t, q), "повтор оставил второе изменение")
}

func TestSendLostAck(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, true)
	go func() {
		assert.Eventually(t, func() bool {
			info, err := stream.Info(context.Background())
			return err == nil && info.State.Msgs == 1
		}, 5*time.Second, 10*time.Millisecond, "изменение не дошло")
		cfg := stream.CachedInfo().Config
		cfg.NoAck = false
		_, err := js.UpdateStream(context.Background(), cfg)
		assert.NoError(t, err, "подтверждения не включены")
	}()
	require.NoError(t, Send(t.Context(), q.Client, change("изменение-2"), q.As(t, anna)), "изменение без подтверждения не повторено")
	assert.Equal(t, uint64(1), count(t, q), "повтор оставил второе изменение")
}

func TestSendWaitsForQueue(t *testing.T) {
	q := queuetest.New(t)
	time.AfterFunc(300*time.Millisecond, func() { q.Start(t, false) })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, Send(ctx, q.Client, change("изменение-3"), q.As(t, anna)), "изменение не дождалось очереди")
	assert.Equal(t, uint64(1), count(t, q), "изменение не в потоке")
}

func TestSendUnavailable(t *testing.T) {
	q := queuetest.New(t)
	began := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	err := Send(ctx, q.Client, change("изменение-4"), q.As(t, anna))
	assert.ErrorIs(t, err, ErrUnavailable, "недоступная очередь не распознана")
	assert.NotErrorIs(t, err, nats.ErrHeadersNotSupported, "отправка до подключения")
	assert.GreaterOrEqual(t, time.Since(began), 300*time.Millisecond, "отказ раньше срока")
}

func TestSendRefusesSettings(t *testing.T) {
	q := queuetest.New(t)
	plain := q.Client
	plain.Queue = "nats" + plain.Queue[len("tls"):]
	broken := q.Client
	broken.QueueCA = "не PEM"
	cases := map[string]struct {
		settings config.Settings
		want     error
	}{
		"plain":     {plain, ErrInsecure},
		"no queue":  {config.Settings{}, ErrNoQueue},
		"broken ca": {broken, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := Send(t.Context(), c.settings, change("изменение-5"))
			require.Error(t, err, "изменение отправлено")
			assert.NotErrorIs(t, err, ErrUnavailable, "клиент пытался подключиться")
			_, cerr := Connect(c.settings)
			require.Error(t, cerr, "подключение с неверными настройками")
			if c.want != nil {
				assert.ErrorIs(t, err, c.want, "не та ошибка настроек")
				assert.ErrorIs(t, cerr, c.want, "не та ошибка настроек подключения")
			}
		})
	}
}

func TestSendDenied(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	for name, extra := range map[string][]nats.Option{
		"без входа":    nil,
		"токен истёк":  {nats.Token(q.Token(t, anna, -time.Minute)), nats.CustomInboxPrefix("_INBOX." + anna)},
		"чужой сервер": {nats.Token("s3cret"), nats.CustomInboxPrefix("_INBOX." + anna)},
	} {
		began := time.Now()
		err := Send(t.Context(), q.Client, change("изменение-6"), extra...)
		assert.ErrorIs(t, err, ErrDenied, "очередь пустила устройство: %s", name)
		assert.NotErrorIs(t, err, ErrUnavailable, "отказ в доступе принят за недоступную очередь: %s", name)
		assert.NotContains(t, err.Error(), "s3cret", "в ошибке токен: %s", name)
		assert.Less(t, time.Since(began), 5*time.Second, "отказ в доступе ждал весь срок: %s", name)
	}
	assert.Zero(t, count(t, q), "изменение без доступа попало в поток")
}

func TestSendForeignOwner(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	err := Send(ctx, q.Client, change("изменение-7"), q.As(t, "0199b5e4-0000-7000-8000-0000000b0715"))
	assert.ErrorIs(t, err, ErrUnavailable, "изменение от чужого имени принято")
	assert.Zero(t, count(t, q), "изменение от чужого имени попало в поток")
}

func TestSendWithoutService(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	err := Send(ctx, q.Client, change("изменение-8"), q.As(t, anna))
	assert.ErrorIs(t, err, ErrUnavailable, "очередь без службы доступа не названа недоступной")
	assert.Zero(t, count(t, q), "изменение без службы доступа попало в поток")
}
