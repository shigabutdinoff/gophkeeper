package worker

import (
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func holding(f *fake) (started chan struct{}, release chan struct{}) {
	started, release = make(chan struct{}, 16), make(chan struct{})
	f.reply = func(n int, w http.ResponseWriter) bool {
		started <- struct{}{}
		<-release
		return false
	}
	return started, release
}

func TestRunFinishesOnStop(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	started, release := holding(f)
	seq := send(t, q, change(t, anna, uuid.NewV7().String(), "s3cret"))
	stop := run(t, js, f.server())
	<-started
	stopped := make(chan error, 1)
	go func() { stopped <- stop() }()
	assert.Never(t, func() bool { return len(stopped) > 0 }, 200*time.Millisecond, 10*time.Millisecond, "обработчик остановился до ответа сервера")
	close(release)
	require.NoError(t, <-stopped, "обработчик остановился с ошибкой")
	acked(t, js, seq)
}

func TestRunGivesUpAfterGrace(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	setVar(t, &Grace, 50*time.Millisecond)
	f := newFake(t)
	started, release := holding(f)
	seq := send(t, q, change(t, anna, uuid.NewV7().String(), "s3cret"))
	stop := run(t, js, f.server())
	<-started
	require.NoError(t, stop(), "обработчик остановился с ошибкой")
	info := consumerInfo(t, js)
	require.NotNil(t, info, "потребитель не найден")
	assert.Less(t, info.AckFloor.Stream, seq, "изменение подтверждено без ответа сервера")
	f.mu.Lock()
	f.reply = nil
	f.mu.Unlock()
	close(release)
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Len(t, rows, 1, "изменение не применено после запуска")
}

func TestRunAfterKill(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	setVar(t, &ackWait, 300*time.Millisecond)
	f := newFake(t)
	first := run(t, js, f.server())
	assert.Eventually(t, func() bool { return consumerInfo(t, js) != nil }, 5*time.Second, 10*time.Millisecond, "потребитель не создан")
	require.NoError(t, first(), "первый запуск не остановился")
	seq := send(t, q, change(t, anna, uuid.NewV7().String(), "1"), change(t, anna, uuid.NewV7().String(), "2"),
		change(t, anna, uuid.NewV7().String(), "3"))
	c, err := js.Consumer(t.Context(), "CHANGES", durable)
	require.NoError(t, err, "потребитель не найден")
	_, err = c.Next()
	require.NoError(t, err, "изменение не взято")
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	var order []string
	for _, c := range f.writes() {
		order = append(order, c.Secret)
	}
	assert.Equal(t, []string{b64("1"), b64("2"), b64("3")}, order, "изменения применены не по одному разу")
	assert.Len(t, rows, 3, "изменение потеряно")
}

func TestRunWaitsForStream(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Run(t, false)
	cfg := stream.CachedInfo().Config
	require.NoError(t, js.DeleteStream(t.Context(), "CHANGES"), "поток не удалён")
	f := newFake(t)
	run(t, js, f.server())
	assert.Eventually(t, func() bool { return len(f.log.lines("очередь изменений")) > 1 }, 5*time.Second, 10*time.Millisecond,
		"ожидание потока не записано в журнал")
	_, err := js.CreateStream(t.Context(), cfg)
	require.NoError(t, err, "поток не создан")
	acked(t, js, send(t, q, change(t, anna, uuid.NewV7().String(), "s3cret")))
}

func TestRunSurvivesQueueRestart(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	f := newFake(t)
	js := queuetest.JetStream(t, q.Admin, nats.ReconnectWait(10*time.Millisecond), nats.MaxReconnects(-1))
	stop := run(t, js, f.server())
	assert.Eventually(t, func() bool { return consumerInfo(t, js) != nil }, 5*time.Second, 10*time.Millisecond, "потребитель не создан")
	q.Server.Shutdown()
	q.Run(t, false)
	acked(t, js, send(t, q, change(t, anna, uuid.NewV7().String(), "s3cret")))
	require.NoError(t, stop(), "обработчик остановился с ошибкой")
}

func TestRunClosedConnection(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	f := newFake(t)
	js := queuetest.JetStream(t, q.Admin)
	done := make(chan error, 1)
	go func() { done <- Run(t.Context(), js, f.server()) }()
	assert.Eventually(t, func() bool { return consumerInfo(t, js) != nil }, 5*time.Second, 10*time.Millisecond, "потребитель не создан")
	js.Conn().Close()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, nats.ErrConnectionClosed, "закрытое соединение не остановило обработчик")
	case <-time.After(5 * time.Second):
		t.Fatal("обработчик не остановился после закрытия соединения")
	}
}
