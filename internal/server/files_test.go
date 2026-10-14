package server

import (
	"bytes"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

func (r *running) files(token string) *storage.Client {
	return storage.New(&http.Client{Timeout: 2 * time.Minute, Transport: r.client().Transport}, "https://"+r.addr, "us-east-1", "app-key", token)
}

type fileServer struct {
	url string
	web http.RoundTripper
	a   *auth
	js  jetstream.JetStream
}

func offlineFiles(t *testing.T, limit int64) *fileServer {
	t.Helper()
	a := offline(t)
	js, _ := queuetest.New(t).Run(t, false)
	s := Settings{AppKey: "app-key", ServerKey: "service-key", Queue: js, FilesLimit: limit}
	var err error
	a.files, err = bucket(t.Context(), s)
	require.NoError(t, err, "корзина файлов не открыта")
	srv := httptest.NewTLSServer(a.routes(s))
	t.Cleanup(srv.Close)
	return &fileServer{url: srv.URL, web: srv.Client().Transport, a: a, js: js}
}

func (f *fileServer) token(t *testing.T, owner string) string {
	t.Helper()
	token, err := f.a.sign(f.url+"/auth/v1", owner, uuid.New().String(), time.Now().Add(lifetime))
	require.NoError(t, err, "токен не подписан")
	return token
}

func (f *fileServer) as(token string) (*storage.Client, *recorder) {
	rec := &recorder{next: f.web}
	return storage.New(&http.Client{Transport: rec, Timeout: time.Minute}, f.url, "us-east-1", "app-key", token), rec
}

func filesBucket(t *testing.T, js jetstream.JetStream) ([]string, jetstream.StreamInfo) {
	t.Helper()
	name := record.Bucket("service-key", "files")
	store, err := js.ObjectStore(t.Context(), name)
	require.NoError(t, err, "корзина файлов не создана")
	objects, err := store.List(t.Context())
	require.NoError(t, err, "корзина файлов пуста, её удалит служба доступа")
	var names []string
	for _, o := range objects {
		names = append(names, o.Name)
	}
	info, err := js.Stream(t.Context(), "OBJ_"+name)
	require.NoError(t, err, "поток корзины файлов не найден")
	return names, *info.CachedInfo()
}

func storedBytes(t *testing.T, js jetstream.JetStream) uint64 {
	t.Helper()
	_, info := filesBucket(t, js)
	return info.State.Bytes
}

func TestFilesMatchSupabase(t *testing.T) {
	f := offlineFiles(t, 1<<30)
	owner, id, missing, data := uuid.New().String(), uuid.New().String(), uuid.New().String(), []byte("шифр файла")
	admin, rec := f.as("service-key")
	require.NoError(t, admin.Put(t.Context(), owner, id, data), "служебный ключ не записал файл")
	assert.Equal(t, reference(t, "files_put.txt"), rec.last(owner, id), "ответ на сохранение не равен эталону")
	got, err := admin.Get(t.Context(), owner, id, 1<<20)
	require.NoError(t, err, "служебный ключ не прочитал файл")
	assert.Equal(t, data, got, "служебный ключ прочитал не то")
	assert.Equal(t, reference(t, "files_get.txt"), rec.last(owner, id), "ответ на выдачу не равен эталону")
	anna, annaRec := f.as(f.token(t, owner))
	got, err = anna.Get(t.Context(), owner, id, 1<<20)
	require.NoError(t, err, "владелец не прочитал свой файл")
	assert.Equal(t, data, got, "владелец прочитал не то")
	assert.Equal(t, reference(t, "files_get.txt"), annaRec.last(owner, id), "ответ владельцу не равен эталону")
	_, err = anna.Get(t.Context(), owner, missing, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "отсутствующий файл не отмечен")
	assert.Equal(t, reference(t, "files_missing.txt"), annaRec.last(owner, missing), "ответ на отсутствующий файл не равен эталону")
	boris, borisRec := f.as(f.token(t, uuid.New().String()))
	_, err = boris.Get(t.Context(), owner, id, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "чужой файл не скрыт")
	assert.Equal(t, reference(t, "files_missing.txt"), borisRec.last(owner, id), "ответ на чужой файл не равен эталону")
	assert.Error(t, anna.Put(t.Context(), owner, id, []byte("подмена")), "пользователь записал файл сам")
	assert.Equal(t, reference(t, "files_put_denied.txt"), annaRec.last(owner, id), "отказ в сохранении не равен эталону")
	assert.Error(t, anna.Delete(t.Context(), owner, id), "пользователь удалил файл сам")
	assert.Equal(t, reference(t, "files_delete_denied.txt"), annaRec.last(owner, id), "отказ в удалении не равен эталону")
	got, err = admin.Get(t.Context(), owner, id, 1<<20)
	assert.True(t, err == nil && bytes.Equal(got, data), "отказы пользователю тронули файл: %v", err)
	for name, c := range map[string]struct{ token, want string }{"испорченный токен": {spoil(f.token(t, owner)), "files_bad_token.txt"},
		"без токена": {"", "files_no_token.txt"}, "чужой ключ": {"app-key", "files_bad_token.txt"}} {
		client, rec := f.as(c.token)
		_, err = client.Get(t.Context(), owner, id, 1<<20)
		assert.Error(t, err, "файл выдан: %s", name)
		assert.Equal(t, reference(t, c.want), rec.last(owner, id), "отказ не равен эталону: %s", name)
	}
	assert.Error(t, rawObject(t, &http.Client{Transport: rec}, f.url, "app-key", "service-key", owner), "путь вне шаблона отдал объект")
	assert.Equal(t, reference(t, "files_unknown.txt"), rec.last(owner, id), "ответ на путь вне шаблона не равен эталону")
	assert.Error(t, rawObject(t, &http.Client{Transport: rec}, f.url, "app-key", "service-key", ".keep"), "метка корзины выдана")
	assert.Equal(t, reference(t, "files_unknown.txt"), strings.Replace(rec.last(".keep", id), "207", "238", 1), "ответ на метку корзины не равен эталону")
	for range 2 {
		require.NoError(t, admin.Delete(t.Context(), owner, id), "служебный ключ не удалил файл")
		assert.Equal(t, reference(t, "files_delete.txt"), rec.last(owner, id), "ответ на удаление не равен эталону")
	}
	_, err = admin.Get(t.Context(), owner, id, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "удалённый файл выдан")
	names, _ := filesBucket(t, f.js)
	assert.Equal(t, []string{".keep"}, names, "в корзине файлов осталось лишнее")
}

func TestFilesTooLarge(t *testing.T) {
	f := offlineFiles(t, 8<<10)
	owner, kept, big := uuid.New().String(), uuid.New().String(), uuid.New().String()
	admin, rec := f.as("service-key")
	require.NoError(t, admin.Put(t.Context(), owner, kept, bytes.Repeat([]byte("ф"), 2<<10)), "файл в пределе не записан")
	assert.Error(t, admin.Put(t.Context(), owner, big, make([]byte, 6<<10)), "файл больше остатка корзины записан")
	assert.Equal(t, reference(t, "files_too_large.txt"), rec.last(owner, big), "отказ полной корзины не равен эталону")
	_, err := admin.Get(t.Context(), owner, big, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "файл, не принятый полной корзиной, выдан")
	assert.Equal(t, reference(t, "files_missing.txt"), rec.last(owner, big), "ответ на непринятый файл не равен эталону")
	before := storedBytes(t, f.js)
	_, err = admin.Get(t.Context(), owner, kept, 1<<20)
	assert.NoError(t, err, "прежний файл не выдан после отказа")
	*rec = recorder{next: rec.next}
	assert.Error(t, admin.Put(t.Context(), owner, big, make([]byte, objectLimit+1)), "файл больше предела Supabase записан")
	assert.Equal(t, reference(t, "files_too_large.txt"), rec.last(owner, big), "отказ файлу больше предела Supabase не равен эталону")
	assert.Equal(t, before, storedBytes(t, f.js), "файл больше предела попал в корзину")
}

func TestFilesBrokenQueue(t *testing.T) {
	f := offlineFiles(t, 1<<30)
	owner, id := uuid.New().String(), uuid.New().String()
	f.js.Conn().Close()
	admin, rec := f.as("service-key")
	for name, call := range map[string]func() error{
		"сохранение": func() error { return admin.Put(t.Context(), owner, id, []byte("файл")) },
		"выдача":     func() error { _, err := admin.Get(t.Context(), owner, id, 1<<20); return err },
		"удаление":   func() error { return admin.Delete(t.Context(), owner, id) },
	} {
		err := call()
		re, ok := errors.AsType[*awshttp.ResponseError](err)
		require.True(t, ok, "сбой очереди не дал ответа сервера: %s: %v", name, err)
		assert.Equal(t, http.StatusInternalServerError, re.HTTPStatusCode(), "сбой очереди не дал 500: %s", name)
		assert.Equal(t, "500\ncontent-length: "+strconv.Itoa(len(rec.body))+"\ncontent-type: text/plain\n"+xmlHead+"files/<owner>/<id></Resource><Code>InternalError</Code><Message>Internal Server Error</Message></Error>",
			rec.last(owner, id), "тело сбоя не как у Supabase: %s", name)
	}
}

func TestFilesSlowUpload(t *testing.T) {
	set(t, &readTimeout, time.Second)
	r := start(t)
	require.Equal(t, http.StatusOK, r.get(t, keysPath, nil).status, "сервер не запустился")
	before, owner := storedBytes(t, r.js), uuid.New().String()
	head := func(id string, n int) string {
		return "PUT /storage/v1/s3/files/" + owner + "/" + id + " HTTP/1.1\r\nHost: localhost\r\nX-Amz-Security-Token: service-key\r\nContent-Length: " +
			strconv.Itoa(n) + "\r\n\r\n"
	}
	slow, cut := uuid.New().String(), uuid.New().String()
	conn := r.dial(t)
	go drip(conn, head(slow, 100), rand.Text()+rand.Text())
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Read(make([]byte, 1))
	var ne net.Error
	assert.False(t, errors.As(err, &ne) && ne.Timeout(), "медленная загрузка не оборвана в срок")
	broken := r.dial(t)
	_, err = broken.Write([]byte(head(cut, 1<<20) + string(make([]byte, 300<<10))))
	require.NoError(t, err, "начало файла не отправлено")
	broken.Close()
	for _, id := range []string{slow, cut} {
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			_, err := r.files("service-key").Get(t.Context(), owner, id, 1<<20)
			assert.ErrorIs(c, err, storage.ErrNoObject, "оборванный файл сохранён")
			assert.Equal(c, before, storedBytes(t, r.js), "оборванная загрузка оставила куски в корзине")
		}, 10*time.Second, 50*time.Millisecond, "загрузка %s не убрана", id)
	}
}

func TestFilesDeleteConfirmed(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	require.NoError(t, r.files("service-key").Put(t.Context(), anna.User.ID, id, []byte("файл")), "файл не сохранён на своём сервере")
	require.NoError(t, r.files("service-key").Delete(t.Context(), anna.User.ID, id), "удаление файла служебным ключом не подтверждено")
	err := r.files("app-key").Delete(t.Context(), anna.User.ID, id)
	re, ok := errors.AsType[*awshttp.ResponseError](err)
	require.True(t, ok, "удаление файла чужим токеном не отклонено: %v", err)
	assert.Equal(t, http.StatusForbidden, re.HTTPStatusCode(), "отказ в удалении файла не 403")
	status, _, _ := r.send(t, http.MethodDelete, "/storage/v1/s3/files/"+anna.User.ID+"/"+id, "", nil)
	assert.Equal(t, http.StatusForbidden, status, "удаление файла без токена сессии не отклонено")
	status, _, _ = r.send(t, http.MethodGet, "/rest/v1/records", "", nil)
	assert.Equal(t, http.StatusUnauthorized, status, "чтение записей без ключа проекта не отклонено")
	kept, ok := r.saved(t, anna.User.ID, id)
	assert.True(t, ok && kept.note == "почта", "удаление файла тронуло запись")
}

func TestFilesNeedRegion(t *testing.T) {
	r := start(t)
	err := storage.New(r.client(), "https://"+r.addr, "", "app-key", "service-key").Delete(t.Context(), uuid.New().String(), uuid.New().String())
	_, answered := errors.AsType[*awshttp.ResponseError](err)
	assert.True(t, err != nil && !answered, "клиент хранилища без региона обратился к серверу: %v", err)
}
