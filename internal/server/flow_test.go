package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
	"github.com/shigabutdinoff/gophkeeper/internal/worker"
)

type journal struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (j *journal) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.buf.Write(p)
}

func (j *journal) String() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.buf.String()
}

type flow struct {
	r    *running
	data string
	log  *journal
}

func startFlow(t *testing.T) *flow {
	t.Helper()
	r := start(t)
	f := &flow{r: r, data: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), log: &journal{}}
	stop, err := access.Serve(t.Context(), r.js.Conn(), r.q.Seed, "", "https://"+f.r.addr, f.r.client(), r.q.Server)
	require.NoError(t, err, "служба доступа не получила ключи сервера")
	t.Cleanup(func() { stop() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	s := worker.Server{URL: "https://" + f.r.addr, Key: "service-key", Client: f.r.client(), Log: log.New(f.log, "", 0), Seed: r.q.Seed,
		Files: f.r.files("service-key")}
	go func() { done <- worker.Run(ctx, r.js, s) }()
	t.Cleanup(func() { cancel(); <-done })
	return f
}

func (f *flow) device(t *testing.T, g granted) jetstream.JetStream {
	t.Helper()
	return queuetest.JetStream(t, f.r.q.Client, func(o *nats.Options) error {
		o.Token, o.InboxPrefix = g.Access, nats.InboxPrefix+g.User.ID
		return nil
	})
}

func (f *flow) send(t *testing.T, js jetstream.JetStream, msg *nats.Msg, err error) {
	t.Helper()
	require.NoError(t, err, "изменение не собрано")
	_, err = js.PublishMsg(t.Context(), msg)
	require.NoError(t, err, "очередь не приняла изменение")
}

func (f *flow) put(t *testing.T, js jetstream.JetStream, owner string, fields map[string]string, note string) string {
	t.Helper()
	msg, id, err := record.Put(f.data, owner, fields, note)
	f.send(t, js, msg, err)
	return id
}

func (f *flow) seen(t *testing.T, g granted) map[string]string {
	t.Helper()
	status, body := f.r.records(t, listQuery, g.Access)
	if status != http.StatusOK {
		return nil
	}
	got := map[string]string{}
	for _, e := range list(t, body) {
		secret, err := base64.StdEncoding.DecodeString(e.Secret)
		require.NoError(t, err, "секрет не в base64")
		fields, err := record.Open(f.data, e.ID, secret)
		require.NoError(t, err, "секрет не расшифрован")
		got[e.ID] = e.Note + " " + fields["login"] + fields["password"] + fields["text"] + fields["number"]
	}
	return got
}

func (f *flow) await(t *testing.T, g granted, want map[string]string, why string) {
	t.Helper()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, want, f.seen(t, g))
	}, 20*time.Second, 50*time.Millisecond, why)
}

func TestFlowThroughQueue(t *testing.T) {
	f := startFlow(t)
	anna, boris := f.r.user(t), f.r.user(t)
	annaJS, borisJS := f.device(t, anna), f.device(t, boris)
	login := f.put(t, annaJS, anna.User.ID, map[string]string{"type": "login", "login": "anna", "password": "s3cret"}, "почта")
	text := f.put(t, annaJS, anna.User.ID, map[string]string{"type": "text", "text": "заметка"}, "текст")
	card := f.put(t, annaJS, anna.User.ID, map[string]string{"type": "card", "number": "4111"}, "карта")
	f.await(t, anna, map[string]string{login: "почта annas3cret", text: "текст заметка", card: "карта 4111"}, "добавления не дошли до сервера")
	msg, err := record.Edit(f.data, anna.User.ID, login, map[string]string{"type": "login", "login": "anna", "password": "new"}, "работа")
	f.send(t, annaJS, msg, err)
	msg, err = record.Delete(anna.User.ID, card)
	f.send(t, annaJS, msg, err)
	msg, err = record.Delete(anna.User.ID, text)
	f.send(t, annaJS, msg, err)
	f.await(t, anna, map[string]string{login: "работа annanew"}, "изменение и удаления не применены")

	msg, err = record.Edit(f.data, boris.User.ID, login, map[string]string{"type": "login", "login": "boris", "password": "x"}, "чужая")
	f.send(t, borisJS, msg, err)
	msg, err = record.Delete(boris.User.ID, login)
	f.send(t, borisJS, msg, err)
	own := f.put(t, borisJS, boris.User.ID, map[string]string{"type": "text", "text": "своё"}, "борис")
	f.await(t, boris, map[string]string{own: "борис своё"}, "очередь встала после изменений Бориса")
	assert.Equal(t, map[string]string{login: "работа annanew"}, f.seen(t, anna), "изменения Бориса тронули запись Анны")
	assert.Equal(t, 2, strings.Count(f.log.String(), "записи нет у владельца"), "обработчик не пропустил изменения Бориса: %s", f.log)

	f.r.stop()
	require.NoError(t, f.r.wait(t), "сервер остановился с ошибкой")
	later := f.put(t, annaJS, anna.User.ID, map[string]string{"type": "text", "text": "потом"}, "позже")
	assert.Eventually(t, func() bool { return strings.Contains(f.log.String(), "connection refused") }, 20*time.Second, 50*time.Millisecond,
		"обработчик не пытался применить изменение при остановленном сервере: %s", f.log)
	f.r.serve(t)
	f.await(t, anna, map[string]string{login: "работа annanew", later: "позже потом"}, "изменение при остановленном сервере не применено после запуска")
}

func (f *flow) file(t *testing.T, g granted, plain []byte) string {
	t.Helper()
	id := uuid.NewV7().String()
	sealed, err := record.SealFile(f.data, id, plain)
	require.NoError(t, err, "файл не зашифрован")
	require.NoError(t, queue.Upload(t.Context(), f.r.q.Client, g.User.ID, id, sealed, nats.Token(g.Access), nats.CustomInboxPrefix(nats.InboxPrefix+g.User.ID)),
		"файл не загружен в очередь")
	msg, err := record.PutFile(f.data, g.User.ID, id, map[string]string{"type": "file", "name": "файл"}, "файл", sealed)
	f.send(t, f.device(t, g), msg, err)
	return id
}

func (f *flow) fetch(t *testing.T, token, owner, id string) ([]byte, error) {
	t.Helper()
	sealed, err := f.r.files(token).Get(t.Context(), owner, id, record.MaxSealed)
	if err != nil {
		return nil, err
	}
	return record.OpenFile(f.data, id, sealed)
}

func (f *flow) awaitFile(t *testing.T, g granted, id string, want []byte) {
	t.Helper()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		got, err := f.fetch(t, g.Access, g.User.ID, id)
		assert.NoError(c, err)
		assert.True(c, bytes.Equal(want, got), "файл выдан не тем")
	}, 2*time.Minute, 100*time.Millisecond, "файл %s не дошёл до сервера", id)
}

func TestFlowFiles(t *testing.T) {
	f := startFlow(t)
	anna, boris := f.r.user(t), f.r.user(t)
	small, big := []byte(rand.Text()), make([]byte, record.MaxFile)
	_, err := rand.Read(big)
	require.NoError(t, err, "большой файл не заполнен")
	smallID, bigID := f.file(t, anna, small), f.file(t, anna, big)
	f.awaitFile(t, anna, smallID, small)
	f.awaitFile(t, anna, bigID, big)
	_, err = f.fetch(t, boris.Access, anna.User.ID, smallID)
	assert.ErrorIs(t, err, storage.ErrNoObject, "Борис получил файл Анны")

	f.r.stop()
	require.NoError(t, f.r.wait(t), "сервер остановился с ошибкой")
	f.r.serve(t)
	f.awaitFile(t, anna, smallID, small)
	f.awaitFile(t, anna, bigID, big)

	msg, err := record.Delete(anna.User.ID, smallID)
	f.send(t, f.device(t, anna), msg, err)
	later := f.put(t, f.device(t, anna), anna.User.ID, map[string]string{"type": "text", "text": "потом"}, "после файла")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := f.fetch(t, "service-key", anna.User.ID, smallID)
		assert.ErrorIs(c, err, storage.ErrNoObject)
		assert.Contains(c, f.seen(t, anna), later)
	}, 20*time.Second, 50*time.Millisecond, "удаление записи не убрало файл или очередь встала")
}

func TestFlowFilesFull(t *testing.T) {
	f := startFlow(t)
	f.r.stop()
	require.NoError(t, f.r.wait(t), "сервер остановился с ошибкой")
	f.r.limit = 64 << 10
	f.r.serve(t)
	anna := f.r.user(t)
	kept := []byte(rand.Text())
	keptID := f.file(t, anna, kept)
	f.awaitFile(t, anna, keptID, kept)
	f.file(t, anna, make([]byte, 100<<10))
	assert.Eventually(t, func() bool { return strings.Contains(f.log.String(), "не применено, повтор") }, 20*time.Second, 50*time.Millisecond,
		"обработчик не повторяет запись с файлом сверх предела корзины: %s", f.log)
	got, err := f.fetch(t, anna.Access, anna.User.ID, keptID)
	assert.True(t, err == nil && bytes.Equal(kept, got), "прежний файл не выдан при полной корзине: %v", err)
}
