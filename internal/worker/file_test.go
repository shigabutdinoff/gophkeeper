package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const (
	otherDigest = "SHA-256=47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU="
	seed        = "ключ-корзин"
)

func named(owner string) string {
	return record.Bucket(seed, owner)
}

func bucket(t *testing.T, js jetstream.JetStream, owner string) jetstream.ObjectStore {
	t.Helper()
	obs, err := js.CreateOrUpdateObjectStore(t.Context(), jetstream.ObjectStoreConfig{Bucket: named(owner)})
	require.NoError(t, err, "корзина владельца не открыта")
	return obs
}

func upload(t *testing.T, js jetstream.JetStream, owner, id, data string) string {
	t.Helper()
	info, err := bucket(t, js, owner).PutBytes(t.Context(), id, []byte(data))
	require.NoError(t, err, "файл не положен в корзину")
	return info.Digest
}

func filed(t *testing.T, owner, id, digest string) *nats.Msg {
	t.Helper()
	msg := nats.NewMsg(record.Topic(owner))
	var err error
	msg.Data, err = json.Marshal(record.Change{Op: "put", Record: id, Note: "пометка", Secret: []byte("s3cret"), Digest: digest})
	require.NoError(t, err, "запись о файле не собрана")
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	return msg
}

func waiting(t *testing.T, js jetstream.JetStream, owner, id string) bool {
	t.Helper()
	obs, err := js.ObjectStore(t.Context(), named(owner))
	require.NoError(t, err, "корзина владельца пропала")
	_, err = obs.GetInfo(t.Context(), id)
	if err != nil {
		require.ErrorIs(t, err, jetstream.ErrObjectNotFound, "объект в корзине не проверен")
	}
	return err == nil
}

func settled(t *testing.T, js jetstream.JetStream, ackWait time.Duration) {
	t.Helper()
	assert.Never(t, func() bool {
		info := consumerInfo(t, js)
		return info == nil || info.NumRedelivered > 0 || info.NumAckPending > 0
	}, 3*ackWait, 50*time.Millisecond, "изменение доставлено повторно")
}

func TestRunMovesFile(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	msg := filed(t, anna, id, upload(t, js, anna, id, "шифр файла"))
	send(t, q, msg)
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	seq := send(t, q, msg)
	run(t, js, f.server())
	acked(t, js, seq)
	trace, objects := f.stored()
	_, rows := f.snapshot()
	assert.Equal(t, map[string]string{anna + "/" + id: "шифр файла"}, objects, "файл не лежит на сервере у владельца")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "запись о файле не создана")
	assert.Equal(t, []string{"PUT " + anna + "/" + id + " служебный-ключ", "POST records"}, trace, "повтор записи дошёл до сервера или порядок не тот")
	assert.False(t, waiting(t, js, anna, id), "объект остался в корзине")
	assert.Len(t, f.log.lines("пропущено"), 1, "повторная доставка не пропущена")
}

func TestRunDropsForgedFile(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{ackWait: 300 * time.Millisecond}
	f := newFake(t)
	id, next := uuid.NewV7().String(), uuid.NewV7().String()
	upload(t, js, anna, id, "подменённый s3cret")
	seq := send(t, q, filed(t, anna, id, otherDigest), change(t, boris, next, "верный"))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	trace, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, objects, "подменённый файл попал на сервер")
	assert.Equal(t, []string{"POST records"}, trace, "сервер тронут подменённым файлом")
	assert.Equal(t, map[[2]string]string{{boris, next}: "пометка/" + b64("верный")}, rows, "запись о подменённом файле создана или очередь встала")
	assert.False(t, waiting(t, js, anna, id), "подменённый файл остался в корзине")
	settled(t, js, tune.ackWait)
	for _, l := range f.log.lines("") {
		assert.NotContains(t, l, "s3cret", "журнал содержит содержимое")
	}
	snaps.MatchSnapshot(t, strings.Join(f.log.lines("отброшено"), "\n"))
}

func TestRunSkipsMissingFile(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	bucket(t, js, boris)
	seq := send(t, q, filed(t, anna, id, otherDigest), filed(t, boris, id, otherDigest))
	run(t, js, f.server())
	acked(t, js, seq)
	trace, _ := f.stored()
	assert.Empty(t, trace, "сервер тронут записью без файла")
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(f.log.lines("пропущено"), "\n"), id, "<id>"))
}

func TestRunRetriesStorage(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{attempt: 100 * time.Millisecond, ackWait: 300 * time.Millisecond}
	f := newFake(t)
	f.files = func(n int, w http.ResponseWriter) bool {
		if n == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if n == 1 {
			time.Sleep(700 * time.Millisecond)
		}
		return n == 0
	}
	id := uuid.NewV7().String()
	seq := send(t, q, filed(t, anna, id, upload(t, js, anna, id, "шифр файла")))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	trace, objects := f.stored()
	_, rows := f.snapshot()
	assert.Equal(t, map[string]string{anna + "/" + id: "шифр файла"}, objects, "файл не дошёл после отказа и медленной записи")
	assert.Len(t, rows, 1, "запись о файле не создана")
	assert.Len(t, trace, 3, "файл записан не с двух попыток или запись повторялась")
	assert.False(t, waiting(t, js, anna, id), "объект остался в корзине")
	assert.Zero(t, consumerInfo(t, js).NumRedelivered, "медленная запись файла дала повторную доставку")
}

func TestRunDropsUnreadableFile(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{maxSealed: 16}
	f := newFake(t)
	huge, bare, next := uuid.NewV7().String(), uuid.NewV7().String(), uuid.NewV7().String()
	meta, err := json.Marshal(jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: bare}, Bucket: named(boris), NUID: "x", Size: 4, Chunks: 1, Digest: otherDigest})
	require.NoError(t, err, "мета объекта не собрана")
	bucket(t, js, boris)
	_, err = js.Publish(t.Context(), "$O."+named(boris)+".M."+base64.URLEncoding.EncodeToString([]byte(bare)), meta)
	require.NoError(t, err, "мета без частей не записана")
	seq := send(t, q, filed(t, anna, huge, upload(t, js, anna, huge, "зашифрованный файл больше предела")),
		filed(t, boris, bare, otherDigest), change(t, anna, next, "верный"))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	trace, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, objects, "нечитаемый файл попал на сервер")
	assert.Equal(t, []string{"POST records"}, trace, "сервер тронут нечитаемым файлом")
	assert.Equal(t, map[[2]string]string{{anna, next}: "пометка/" + b64("верный")}, rows, "следующее изменение не применено")
	assert.False(t, waiting(t, js, anna, huge), "файл больше предела остался в корзине")
	assert.False(t, waiting(t, js, boris, bare), "объект без частей остался в корзине")
	snaps.MatchSnapshot(t, strings.Join(f.log.lines("отброшено"), "\n"))
}

func TestRunDropsStreamReply(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	forged := operation(t, anna, "delete", id, "", "")
	forged.Header.Set(jetstream.StreamHeader, "OBJ_"+boris)
	bare := operation(t, anna, "delete", id, "", "")
	bare.Header.Del(nats.MsgIdHdr)
	seq := send(t, q, change(t, anna, id, "s3cret"), forged, bare)
	run(t, js, f.server())
	acked(t, js, seq)
	trace, _ := f.stored()
	_, rows := f.snapshot()
	assert.Equal(t, []string{"POST records"}, trace, "сервер тронут ответом чтения из потока")
	assert.Len(t, rows, 1, "запись Анны удалена ответом чтения из потока")
	snaps.MatchSnapshot(t, strings.Join(f.log.lines("отброшено"), "\n"))
}

func TestRunDeletesFile(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id, login := uuid.NewV7().String(), uuid.NewV7().String()
	seq := send(t, q, filed(t, anna, id, upload(t, js, anna, id, "шифр файла")), change(t, anna, login, "s3cret"),
		operation(t, anna, "edit", login, "почта", "n3w"), operation(t, anna, "delete", id, "", ""), operation(t, anna, "delete", login, "", ""))
	run(t, js, f.server())
	acked(t, js, seq)
	trace, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, objects, "файл удалённой записи остался на сервере")
	assert.Empty(t, rows, "записи не удалены")
	assert.Equal(t, []string{"PUT " + anna + "/" + id + " служебный-ключ", "POST records", "POST records", "PATCH records",
		"DELETE " + anna + "/" + id + " служебный-ключ", "DELETE records", "DELETE " + anna + "/" + login + " служебный-ключ", "DELETE records"},
		trace, "объект удалён не раньше строки или сервер тронут лишний раз")
}

func forgedMeta(t *testing.T, js jetstream.JetStream, info jetstream.ObjectInfo) {
	t.Helper()
	meta, err := json.Marshal(info)
	require.NoError(t, err, "мета объекта не собрана")
	_, err = js.Publish(t.Context(), "$O."+info.Bucket+".M."+base64.URLEncoding.EncodeToString([]byte(info.Name)), meta)
	require.NoError(t, err, "мета объекта не записана")
}

func TestRunDropsMetaWithoutNUID(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	empty, short, next := uuid.NewV7().String(), uuid.NewV7().String(), uuid.NewV7().String()
	bucket(t, js, anna)
	bucket(t, js, boris)
	forgedMeta(t, js, jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: empty}, Bucket: named(anna), Digest: otherDigest})
	forgedMeta(t, js, jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: short}, Bucket: named(boris), Size: 4, Chunks: 1, Digest: otherDigest})
	seq := send(t, q, filed(t, anna, empty, otherDigest), filed(t, boris, short, otherDigest), change(t, anna, next, "верный"))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Equal(t, map[[2]string]string{{anna, next}: "пометка/" + b64("верный")}, rows, "мета без NUID остановила очередь или создала запись")
	assert.Len(t, f.log.lines("отброшено"), 2, "мета без NUID не отброшена как подделка")
}

func TestRunDropsLinkToForeignBucket(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	victim, id := uuid.NewV7().String(), uuid.NewV7().String()
	digest := upload(t, js, anna, victim, "шифр файла Анны")
	bucket(t, js, boris)
	forgedMeta(t, js, jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: id, Opts: &jetstream.ObjectMetaOptions{
		Link: &jetstream.ObjectLink{Bucket: named(anna), Name: victim}}}, Bucket: named(boris), NUID: "x", Digest: digest})
	seq := send(t, q, filed(t, boris, id, digest))
	run(t, js, f.server())
	acked(t, js, seq)
	_, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, objects, "файл из чужой корзины лёг на сервер у другого владельца")
	assert.Empty(t, rows, "запись со ссылкой на чужой файл создана")
	assert.True(t, waiting(t, js, anna, victim), "файл Анны пропал из её корзины")
}

func TestRunDropsUnderstatedSize(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{maxSealed: 16}
	f := newFake(t)
	id, next := uuid.NewV7().String(), uuid.NewV7().String()
	digest := upload(t, js, anna, id, "зашифрованный файл больше предела")
	real, err := bucket(t, js, anna).GetInfo(t.Context(), id)
	require.NoError(t, err, "сведения об объекте не получены")
	real.Size = 8
	forgedMeta(t, js, *real)
	seq := send(t, q, filed(t, anna, id, digest), change(t, boris, next, "верный"))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	_, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, objects, "файл больше предела с заниженным размером в мете лёг на сервер")
	assert.Equal(t, map[[2]string]string{{boris, next}: "пометка/" + b64("верный")}, rows, "запись о файле больше предела создана или очередь встала")
}

type counted struct {
	jetstream.ObjectStore
	read *int
}

type countedResult struct {
	jetstream.ObjectResult
	read *int
}

func (c counted) Get(ctx context.Context, name string, opts ...jetstream.GetObjectOpt) (jetstream.ObjectResult, error) {
	res, err := c.ObjectStore.Get(ctx, name, opts...)
	return countedResult{res, c.read}, err
}

func (r countedResult) Read(p []byte) (int, error) {
	n, err := r.ObjectResult.Read(p)
	*r.read += n
	return n, err
}

func TestReadStopsAtLimit(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	const limit = 16
	id := uuid.NewV7().String()
	digest := upload(t, js, anna, id, "зашифрованный файл больше предела")
	var got int
	obs := counted{bucket(t, js, anna), &got}
	meta, err := obs.GetInfo(t.Context(), id)
	require.NoError(t, err, "сведения об объекте не получены")
	meta.Size = 8
	_, err = read(t.Context(), js, obs, named(anna), meta, record.Change{Record: id, Digest: digest}, limit)
	require.ErrorIs(t, err, record.ErrForged, "файл больше предела не отброшен")
	assert.EqualValues(t, limit+1, got, "из очереди прочитано не столько, сколько позволяет предел")
}

func TestRunSkipDeletesStorageOrphan(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	f.reply = func(n int, w http.ResponseWriter) bool {
		if n == 0 {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("[]"))
			return true
		}
		return false
	}
	id := uuid.NewV7().String()
	seq := send(t, q, filed(t, anna, id, upload(t, js, anna, id, "шифр файла")))
	run(t, js, f.server())
	acked(t, js, seq)
	_, objects := f.stored()
	_, rows := f.snapshot()
	assert.Empty(t, rows, "запись без владельца создана")
	assert.Empty(t, objects, "объект остался в Storage без записи")
	assert.False(t, waiting(t, js, anna, id), "объект остался в корзине")
	assert.Len(t, f.log.lines("пропущено"), 1, "пропуск не записан в журнал")
}
