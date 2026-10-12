package access_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const (
	carol  = "0199b5e4-0000-7000-8000-0000000ca501"
	object = "0199b5e4-7c3a-7d21-9f4e-2a6c8b1d3e57"
)

func device(t *testing.T, q *queuetest.Queue, owner string) (*nats.Conn, chan string) {
	t.Helper()
	errs := make(chan string, 32)
	nc, err := dial(t, q, q.As(t, owner), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		errs <- err.Error()
	}))
	require.NoError(t, err, "вошедшее устройство не подключилось")
	return nc, errs
}

func openBucket(t *testing.T, nc *nats.Conn, owner string) string {
	t.Helper()
	msg, err := nc.Request(record.FilesTopic(owner), nil, 5*time.Second)
	require.NoError(t, err, "корзина %s не открыта", owner)
	return string(msg.Data)
}

func putObject(t *testing.T, nc *nats.Conn, bucket string, data []byte) *jetstream.ObjectInfo {
	t.Helper()
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	obs, err := js.ObjectStore(t.Context(), bucket)
	require.NoError(t, err, "корзина %s не найдена", bucket)
	info, err := obs.PutBytes(t.Context(), object, data)
	require.NoError(t, err, "объект не записан в корзину %s", bucket)
	return info
}

func refusals(t *testing.T, nc *nats.Conn, errs chan string, want int) []string {
	t.Helper()
	require.NoError(t, nc.Flush(), "очередь не ответила")
	var refused []string
	for len(refused) < want {
		select {
		case reason := <-errs:
			refused = append(refused, reason)
		case <-time.After(2 * time.Second):
			t.Fatalf("отказов %d, а не %d: %v", len(refused), want, refused)
		}
	}
	select {
	case reason := <-errs:
		t.Fatalf("лишний отказ: %s", reason)
	case <-time.After(100 * time.Millisecond):
	}
	return refused
}

func exists(t *testing.T, js jetstream.JetStream, bucket string) bool {
	t.Helper()
	_, err := js.ObjectStore(t.Context(), bucket)
	if err != nil {
		require.ErrorIs(t, err, jetstream.ErrBucketNotFound, "корзина %s не проверена", bucket)
	}
	return err == nil
}

func TestBucketOwnerOnly(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	own, ownErrs := device(t, q, anna)
	name := openBucket(t, own, anna)
	assert.Equal(t, record.Bucket(q.Seed, anna), name, "имя корзины выведено не из ключа службы")
	assert.NotContains(t, name, anna, "имя корзины выдаёт владельца")
	info := putObject(t, own, name, []byte("шифр файла"))
	assert.Equal(t, []uint64{19, 1}, []uint64{info.Size, uint64(info.Chunks)}, "объект записан не целиком")

	stranger, errs := device(t, q, boris)
	bucket := "OBJ_" + name
	for _, subject := range []string{record.FilesTopic(anna), "$JS.API.STREAM.INFO." + bucket, "$JS.API.STREAM.MSG.GET." + bucket,
		"$JS.API.DIRECT.GET." + bucket, "$JS.API.DIRECT.GET." + bucket + ".$O." + name + ".M.x", "$JS.API.CONSUMER.CREATE." + bucket,
		"$JS.API.STREAM.PURGE." + bucket, "$O." + name + ".C.x", "$O." + name + ".M.x", "$JS.API.STREAM.NAMES", "$JS.API.STREAM.LIST"} {
		require.NoError(t, stranger.Publish(subject, []byte("чужое")), "публикация в %s не отправлена", subject)
	}
	_, err := stranger.SubscribeSync("$O." + name + ".>")
	require.NoError(t, err, "подписка на чужую корзину не отправлена")
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(refusals(t, stranger, errs, 12), "\n"), name, "<корзина>"))

	for _, api := range []string{"CREATE", "UPDATE", "DELETE"} {
		require.NoError(t, own.Publish("$JS.API.STREAM."+api+"."+bucket, []byte("{}")), "запрос %s не отправлен", api)
	}
	require.NoError(t, own.Publish("$JS.API.STREAM.CREATE.OBJ_x", []byte("{}")), "запрос создания потока не отправлен")
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(refusals(t, own, ownErrs, 4), "\n"), name, "<корзина>"))

	obs, err := js.ObjectStore(t.Context(), name)
	require.NoError(t, err, "корзина владельца пропала")
	got, err := obs.GetBytes(t.Context(), object)
	require.NoError(t, err, "объект владельца не читается")
	assert.Equal(t, "шифр файла", string(got), "объект владельца изменён")
	changes, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	assert.Zero(t, changes.State.Msgs, "запрос открытия или файл попали в поток изменений")
}

func TestSweep(t *testing.T) {
	access.SetIdle(t, 300*time.Millisecond)
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	first, _ := device(t, q, anna)
	annas := openBucket(t, first, anna)
	putObject(t, first, annas, []byte("применённый файл"))
	obs, err := js.ObjectStore(t.Context(), annas)
	require.NoError(t, err, "корзина Анны не найдена")
	require.NoError(t, obs.Delete(t.Context(), object), "объект Анны не удалён")

	second, _ := device(t, q, carol)
	carols := openBucket(t, second, carol)
	assert.True(t, exists(t, js, annas), "убрана корзина, в которую только что писали")
	putObject(t, second, carols, []byte("файл ждёт сервер"))

	time.Sleep(400 * time.Millisecond)
	third, _ := device(t, q, boris)
	boriss := openBucket(t, third, boris)
	assert.False(t, exists(t, js, annas), "пустая простаивающая корзина не убрана")
	assert.True(t, exists(t, js, carols), "убрана корзина с живым объектом")
	assert.True(t, exists(t, js, boriss), "корзина Бориса не открыта")
	putObject(t, third, openBucket(t, third, boris), []byte("файл Бориса"))
}

func TestReplyIntoForeignTopic(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	nc, _ := device(t, q, boris)
	name := openBucket(t, nc, boris)
	forged, err := record.Delete(anna, object)
	require.NoError(t, err, "подделка не собрана")
	chunk := "$O." + name + ".C.x"
	_, err = nc.Request(chunk, forged.Data, 5*time.Second)
	require.NoError(t, err, "подделка не записана в свою корзину")
	require.NoError(t, nc.PublishRequest("$JS.API.DIRECT.GET.OBJ_"+name+"."+chunk, record.Topic(anna), nil), "запрос чтения не отправлен")
	var got *jetstream.RawStreamMsg
	require.Eventually(t, func() bool {
		got, err = stream.GetLastMsgForSubject(t.Context(), record.Topic(anna))
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "ответ чтения не попал в тему изменений Анны")
	assert.Equal(t, string(forged.Data), string(got.Data), "в теме Анны не подделка")
	assert.Equal(t, "OBJ_"+name, got.Header.Get("Nats-Stream"), "у ответа чтения нет заголовка потока")
}

func TestOpenRepliesOnlyToOwnerInbox(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	nc, _ := device(t, q, anna)
	require.NoError(t, nc.PublishRequest(record.FilesTopic(anna), "$JS.API.STREAM.DELETE.CHANGES", nil), "запрос открытия не отправлен")
	require.NoError(t, nc.Flush(), "очередь не ответила")
	assert.Never(t, func() bool {
		_, err := js.Stream(t.Context(), "CHANGES")
		return err != nil
	}, time.Second, 50*time.Millisecond, "ответ службы корзин исполнен как команда администратора, поток изменений удалён")
	assert.False(t, exists(t, js, record.Bucket(q.Seed, anna)), "запрос с чужой темой ответа открыл корзину")
}

func TestSweepKeepsReopenedBucket(t *testing.T) {
	access.SetIdle(t, 300*time.Millisecond)
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	first, _ := device(t, q, anna)
	name := openBucket(t, first, anna)
	putObject(t, first, name, []byte("применённый файл"))
	obs, err := js.ObjectStore(t.Context(), name)
	require.NoError(t, err, "корзина Анны не найдена")
	require.NoError(t, obs.Delete(t.Context(), object), "объект Анны не удалён")
	time.Sleep(400 * time.Millisecond)
	openBucket(t, first, anna)
	second, _ := device(t, q, boris)
	openBucket(t, second, boris)
	assert.True(t, exists(t, js, name), "корзина, которую владелец только что открыл для загрузки, убрана чужим запросом")
	putObject(t, first, name, []byte("новый файл Анны"))
}

func TestOpenIgnoresReflectedReply(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	nc, _ := device(t, q, boris)
	require.NoError(t, nc.PublishRequest(record.Topic(boris), record.FilesTopic(carol), []byte("{}")), "изменение не отправлено")
	require.NoError(t, nc.Flush(), "очередь не ответила")
	assert.Never(t, func() bool {
		_, err := js.ObjectStore(t.Context(), record.Bucket(q.Seed, carol))
		return err == nil
	}, time.Second, 50*time.Millisecond, "ответ очереди в тему открытия завёл корзину без запроса владельца")
}

func TestReflectedReplySparesForeignBucket(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	own, _ := device(t, q, anna)
	name := openBucket(t, own, anna)
	putObject(t, own, name, []byte("шифр файла"))
	nc, _ := device(t, q, boris)
	mine := openBucket(t, nc, boris)
	roll := nats.NewMsg("$O." + mine + ".C.x")
	roll.Header.Set("Nats-Rollup", "all")
	_, err := nc.RequestMsg(roll, 5*time.Second)
	require.NoError(t, err, "сообщение со свёрткой не записано в свою корзину")
	for _, reply := range []string{"$O." + anna + ".C.y", "$O." + anna + ".M.y"} {
		require.NoError(t, nc.PublishRequest("$JS.API.DIRECT.GET.OBJ_"+mine+"."+roll.Subject, reply, nil), "запрос чтения не отправлен")
	}
	require.NoError(t, nc.Flush(), "очередь не ответила")
	obs, err := js.ObjectStore(t.Context(), name)
	require.NoError(t, err, "корзина владельца пропала")
	assert.Never(t, func() bool {
		_, err := obs.GetBytes(t.Context(), object)
		return err != nil
	}, time.Second, 50*time.Millisecond, "ответ чтения с чужой темой ответа стёр файл владельца")
}
