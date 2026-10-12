package queue

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const (
	boris  = "0199b5e4-0000-7000-8000-0000000b0715"
	object = "0199b5e4-7c3a-7d21-9f4e-2a6c8b1d3e57"
)

func TestUpload(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	data := bytes.Repeat([]byte("шифр"), 1<<16)
	require.NoError(t, Upload(t.Context(), q.Client, anna, object, data, q.As(t, anna)), "файл не отправлен в очередь, хотя сервера нет и не нужно")
	obs, err := js.ObjectStore(t.Context(), record.Bucket(q.Seed, anna))
	require.NoError(t, err, "корзина владельца не открыта")
	got, err := obs.GetBytes(t.Context(), object)
	require.NoError(t, err, "объект не читается из корзины")
	assert.True(t, bytes.Equal(data, got), "в корзине не те байты")
	info, err := obs.GetInfo(t.Context(), object)
	require.NoError(t, err, "сведения об объекте не получены")
	assert.Equal(t, record.Digest(data), info.Digest, "сумма объекта в очереди не равна сумме записи")
	assert.Zero(t, count(t, q), "запрос открытия или файл попали в поток изменений")
}

func TestUploadDenied(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	began := time.Now()
	err := Upload(t.Context(), q.Client, anna, object, []byte("шифр"))
	assert.ErrorIs(t, err, ErrDenied, "очередь пустила устройство без входа")
	assert.Less(t, time.Since(began), 5*time.Second, "отказ в доступе ждал весь срок")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	err = Upload(ctx, q.Client, anna, object, []byte("шифр"), q.As(t, boris))
	assert.ErrorIs(t, err, ErrUnavailable, "файл принят в чужую корзину")
	_, err = js.ObjectStore(t.Context(), record.Bucket(q.Seed, anna))
	assert.ErrorIs(t, err, jetstream.ErrBucketNotFound, "чужой вход открыл корзину владельца")
}

func TestUploadUnavailable(t *testing.T) {
	q := queuetest.New(t)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	err := Upload(ctx, q.Client, anna, object, []byte("шифр"), q.As(t, anna))
	assert.ErrorIs(t, err, ErrUnavailable, "закрытая очередь не названа недоступной")
	err = Upload(t.Context(), config.Settings{}, anna, object, []byte("шифр"))
	assert.ErrorIs(t, err, ErrNoQueue, "файл отправлен без адреса очереди")
}

func TestUploadFull(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	_, err := js.CreateObjectStore(t.Context(), jetstream.ObjectStoreConfig{Bucket: "tight", MaxBytes: 1 << 10})
	require.NoError(t, err, "тесная корзина не создана")
	_, err = js.Conn().Subscribe("files.open.*", func(msg *nats.Msg) {
		msg.Respond(map[string][]byte{record.FilesTopic(anna): []byte("tight")}[msg.Subject])
	})
	require.NoError(t, err, "подписка открытия не создана")
	err = Upload(t.Context(), q.Admin, anna, object, make([]byte, 1<<12))
	assert.ErrorIs(t, err, ErrUnavailable, "файл принят в переполненную корзину")
	err = Upload(t.Context(), q.Admin, boris, object, []byte("шифр"))
	assert.ErrorIs(t, err, ErrUnavailable, "файл принят без корзины")
}
