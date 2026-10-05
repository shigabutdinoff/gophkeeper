package queue

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const inbox = "_INBOX.pub"

type testQueue struct {
	settings config.Settings
	opts     *server.Options
	js       jetstream.JetStream
	stream   jetstream.Stream
}

func newQueue(t *testing.T) testQueue {
	t.Helper()
	port := freePort(t)
	web := httptest.NewTLSServer(http.NotFoundHandler())
	web.Close()
	opts := &server.Options{
		Host: "127.0.0.1", Port: port, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		TLSConfig: &tls.Config{Certificates: web.TLS.Certificates, MinVersion: tls.VersionTLS12},
		Users: []*server.User{{Username: "admin", Password: "admin-pass"}, {
			Username: "client", Password: "client-pass", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"changes.>"}},
				Subscribe: &server.SubjectPermission{Allow: []string{inbox + ".>"}},
			},
		}},
	}
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: web.Certificate().Raw}))
	return testQueue{opts: opts, settings: config.Settings{
		Queue: "tls://" + net.JoinHostPort(opts.Host, strconv.Itoa(port)), QueueUser: "client",
		QueuePass: "client-pass", QueueInbox: inbox, QueueCA: ca,
	}}
}

func (q *testQueue) start(t *testing.T, noAck bool) {
	t.Helper()
	s := natstest.RunServer(q.opts)
	t.Cleanup(s.Shutdown)
	admin := q.settings
	admin.QueueUser, admin.QueuePass, admin.QueueInbox = "admin", "admin-pass", "_INBOX.adm"
	opts, err := options(admin)
	require.NoError(t, err, "настройки администратора не собраны")
	nc, err := nats.Connect(admin.Queue, opts...)
	require.NoError(t, err, "администратор не подключился")
	t.Cleanup(nc.Close)
	q.js, err = jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	q.stream, err = q.js.CreateStream(t.Context(), jetstream.StreamConfig{
		Name: "CHANGES", Subjects: []string{"changes.>"}, Storage: jetstream.MemoryStorage,
		Duplicates: 2 * time.Minute, NoAck: noAck,
	})
	require.NoError(t, err, "поток CHANGES не создан")
}

func (q *testQueue) count(t *testing.T) uint64 {
	t.Helper()
	info, err := q.stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	return info.State.Msgs
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "свободный порт не найден")
	require.NoError(t, l.Close(), "порт не освобождён")
	return l.Addr().(*net.TCPAddr).Port
}

func change(id string) *nats.Msg {
	msg := nats.NewMsg("changes.records")
	msg.Data = []byte("изменение")
	msg.Header.Set(nats.MsgIdHdr, id)
	return msg
}

func shortAttempt(t *testing.T) {
	old := attempt
	attempt = 200 * time.Millisecond
	t.Cleanup(func() { attempt = old })
}

func TestSendOnce(t *testing.T) {
	q := newQueue(t)
	q.start(t, false)
	msg := change("изменение-1")
	require.NoError(t, Send(t.Context(), q.settings, msg), "изменение не отправлено")
	require.NoError(t, Send(t.Context(), q.settings, msg), "повтор изменения не отправлен")
	assert.Equal(t, uint64(1), q.count(t), "повтор оставил второе изменение")
}

func TestSendLostAck(t *testing.T) {
	shortAttempt(t)
	q := newQueue(t)
	q.start(t, true)
	go func() {
		assert.Eventually(t, func() bool {
			info, err := q.stream.Info(context.Background())
			return err == nil && info.State.Msgs == 1
		}, 5*time.Second, 10*time.Millisecond, "изменение не дошло")
		cfg := q.stream.CachedInfo().Config
		cfg.NoAck = false
		_, err := q.js.UpdateStream(context.Background(), cfg)
		assert.NoError(t, err, "подтверждения не включены")
	}()
	require.NoError(t, Send(t.Context(), q.settings, change("изменение-2")), "изменение без подтверждения не повторено")
	assert.Equal(t, uint64(1), q.count(t), "повтор оставил второе изменение")
}

func TestSendWaitsForQueue(t *testing.T) {
	q := newQueue(t)
	time.AfterFunc(300*time.Millisecond, func() { q.start(t, false) })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, Send(ctx, q.settings, change("изменение-3")), "изменение не дождалось очереди")
	assert.Equal(t, uint64(1), q.count(t), "изменение не в потоке")
}

func TestSendUnavailable(t *testing.T) {
	q := newQueue(t)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	began := time.Now()
	err := Send(ctx, q.settings, change("изменение-4"))
	assert.ErrorIs(t, err, ErrUnavailable, "недоступная очередь не распознана")
	assert.NotErrorIs(t, err, nats.ErrHeadersNotSupported, "отправка до подключения")
	assert.GreaterOrEqual(t, time.Since(began), 300*time.Millisecond, "отказ раньше срока")
}

func TestSendRefusesSettings(t *testing.T) {
	q := newQueue(t)
	plain := q.settings
	plain.Queue = "nats" + plain.Queue[len("tls"):]
	broken := q.settings
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
			if c.want != nil {
				assert.ErrorIs(t, err, c.want, "не та ошибка настроек")
			}
		})
	}
}
