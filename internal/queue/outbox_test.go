package queue

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func dataKey() string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
}

func device(t *testing.T) *queuetest.Queue {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return queuetest.New(t)
}

func open(t *testing.T, q *queuetest.Queue, user, access string) *Outbox {
	t.Helper()
	o, err := Open(t.Context(), q.Client, user, dataKey(), access)
	require.NoError(t, err, "очередь устройства не открыта")
	return o
}

func kept(t *testing.T, q *queuetest.Queue, user string) {
	t.Helper()
	o := open(t, q, user, "")
	defer o.Close()
	require.NoError(t, o.Put(t.Context(), change("изменение-1")), "изменение не сохранено на устройстве")
}

func TestOutboxSendsOnce(t *testing.T) {
	q := device(t)
	q.Start(t, false)
	o := open(t, q, anna, q.Token(t, anna, time.Hour))
	for range 2 {
		require.NoError(t, o.Put(t.Context(), change("изменение-1")), "изменение не сохранено на устройстве")
		require.NoError(t, o.Drain(t.Context()), "изменение не отправлено")
	}
	o.Close()
	assert.Equal(t, uint64(1), count(t, q), "повтор оставил второе изменение")
	stream, err := queuetest.JetStream(t, q.Admin).Stream(t.Context(), "CHANGES")
	require.NoError(t, err, "поток не найден")
	msg, err := stream.GetMsg(t.Context(), 1)
	require.NoError(t, err, "изменение не прочитано")
	assert.Equal(t, nats.Header{nats.MsgIdHdr: {"изменение-1"}}, msg.Header, "в изменении лишние заголовки")
	assert.Equal(t, []string{"changes.records." + anna, "изменение"}, []string{msg.Subject, string(msg.Data)}, "изменение искажено")
	assert.False(t, Kept(), "отправленное изменение осталось на устройстве")
}

func TestOutboxWaitsForCommand(t *testing.T) {
	q := device(t)
	first := open(t, q, anna, "")
	opened := make(chan *Outbox)
	go func() {
		o, err := Open(context.Background(), q.Client, anna, dataKey(), "")
		assert.NoError(t, err, "вторая команда не открыла очередь")
		opened <- o
	}()
	select {
	case o := <-opened:
		o.Close()
		t.Fatal("вторая команда не дождалась первой")
	case <-time.After(300 * time.Millisecond):
	}
	first.Close()
	select {
	case o := <-opened:
		o.Close()
	case <-time.After(10 * time.Second):
		t.Fatal("вторая команда не получила очередь после первой")
	}
}

func TestOutboxPerUser(t *testing.T) {
	q := device(t)
	kept(t, q, anna)
	o := open(t, q, boris, "")
	require.NoError(t, o.Drain(t.Context()), "пустая очередь Бориса не выгружена")
	o.Close()
	assert.True(t, Kept(), "очередь Анны удалена")
	o = open(t, q, anna, "")
	defer o.Close()
	assert.ErrorIs(t, o.Drain(t.Context()), ErrDenied, "очередь Анны пуста или отправлена без токена")
}

func TestOutboxDenied(t *testing.T) {
	q := device(t)
	q.Start(t, false)
	kept(t, q, anna)
	o := open(t, q, anna, q.Token(t, anna, -time.Minute))
	defer o.Close()
	start := time.Now()
	assert.ErrorIs(t, o.Drain(t.Context()), ErrDenied, "отказ очереди не распознан")
	assert.Less(t, time.Since(start), leafWait, "отказ ждал срока связи")
	assert.Zero(t, count(t, q), "изменение прошло с истёкшим токеном")
}

func TestOutboxUnavailable(t *testing.T) {
	q := device(t)
	kept(t, q, anna)
	o := open(t, q, anna, q.Token(t, anna, time.Hour))
	defer o.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, o.Drain(ctx), ErrUnavailable, "недоступная очередь не распознана")
}

func TestFlush(t *testing.T) {
	q := device(t)
	left, err := Flush(t.Context(), q.Client, anna, dataKey(), "токен")
	require.NoError(t, err, "пустое устройство не выгружено")
	assert.Empty(t, left, "у пустого устройства есть изменения")
	assert.False(t, Kept(), "выгрузка пустого устройства создала очередь")
	kept(t, q, anna)
	left, err = Flush(t.Context(), config.Settings{}, anna, dataKey(), "токен")
	require.NoError(t, err, "выгрузка без адреса не пропущена")
	assert.Empty(t, left, "без адреса прочитаны изменения")
	q.Start(t, false)
	left, err = Flush(t.Context(), q.Client, anna, dataKey(), q.Token(t, anna, time.Hour))
	require.NoError(t, err, "изменение не выгружено")
	assert.Empty(t, left, "выгруженное изменение осталось в остатке")
	assert.Equal(t, uint64(1), count(t, q), "в очереди не одно изменение")
	assert.False(t, Kept(), "выгруженное изменение осталось на устройстве")
}

func keep(t *testing.T, q *queuetest.Queue, msgs ...*nats.Msg) {
	t.Helper()
	o := open(t, q, anna, "")
	defer o.Close()
	for _, msg := range msgs {
		require.NoError(t, o.Put(t.Context(), msg), "изменение не сохранено на устройстве")
	}
}

func put(t *testing.T, note string) (*nats.Msg, string) {
	t.Helper()
	msg, id, err := record.Put(dataKey(), anna, map[string]string{"login": "anna", "password": "s3cret"}, note)
	require.NoError(t, err, "запись не собрана")
	return msg, id
}

func TestFlushLeftEdit(t *testing.T) {
	q := device(t)
	added, id := put(t, "старая")
	edited, err := record.Edit(dataKey(), anna, id, map[string]string{"login": "anna", "password": "new"}, "новая")
	require.NoError(t, err, "правка не собрана")
	keep(t, q, added, edited)
	left, err := Flush(t.Context(), q.Client, anna, dataKey(), "")
	assert.ErrorIs(t, err, ErrDenied, "без токена изменения отправлены")
	require.Len(t, left, 1, "в остатке не одна запись")
	assert.Equal(t, []string{"edit", "новая"}, []string{left[id].Op, left[id].Note}, "в остатке не последняя правка")
	assert.True(t, Kept(), "неотправленные изменения удалены")
}

func TestFlushLeftDelete(t *testing.T) {
	q := device(t)
	added, id := put(t, "удалить")
	deleted, err := record.Delete(anna, id)
	require.NoError(t, err, "удаление не собрано")
	other, otherID := put(t, "другая")
	keep(t, q, added, change("неразборчивое"), deleted, other)
	left, _ := Flush(t.Context(), q.Client, anna, dataKey(), "")
	assert.Len(t, left, 2, "в остатке не две записи")
	assert.Equal(t, "delete", left[id].Op, "удаление не последнее изменение")
	assert.Equal(t, "put", left[otherID].Op, "вторая запись потеряна")
}

func TestFlushLeftUnavailable(t *testing.T) {
	q := device(t)
	added, id := put(t, "без связи")
	keep(t, q, added)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	left, err := Flush(ctx, q.Client, anna, dataKey(), q.Token(t, anna, time.Hour))
	assert.ErrorIs(t, err, ErrUnavailable, "недоступная очередь не распознана")
	assert.Equal(t, "без связи", left[id].Note, "остаток не прочитан после срока")
}

func TestOpenRefuses(t *testing.T) {
	q := device(t)
	_, err := Open(t.Context(), q.Client, anna, "не base64!", "")
	assert.Error(t, err, "очередь открыта с негодным ключом")
	s := q.Client
	s.QueueLeaf = "tls://%zz"
	_, err = Open(t.Context(), s, anna, dataKey(), "токен")
	assert.Error(t, err, "очередь открыта с негодным адресом")
	s.QueueLeaf, s.QueueCA = q.Client.QueueLeaf, "не PEM"
	_, err = Open(t.Context(), s, anna, dataKey(), "токен")
	assert.Error(t, err, "очередь открыта с негодным сертификатом")
	assert.ErrorIs(t, Check(config.Settings{Queue: q.Client.Queue}), ErrNoLeaf, "проверка пропустила пустой адрес устройства")
}
