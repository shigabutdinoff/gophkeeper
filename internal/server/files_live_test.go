package server

import (
	"context"
	"crypto/rand"
	"net/http"
	"os"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

func (l liveSupabase) member(t *testing.T, name string) granted {
	t.Helper()
	email := name + "+gop9-" + time.Now().Format("20060102150405.000000") + "@example.com"
	status, body := l.do(t, http.MethodPost, "auth/v1/signup", "", map[string]string{"email": email, "password": rand.Text()})
	u := parse(t, body)
	require.NotEmpty(t, u.Access, "регистрация %s не выдала токен, ответ %d: %s", name, status, body)
	t.Cleanup(func() {
		gone, body := l.do(t, http.MethodDelete, "auth/v1/admin/users/"+u.User.ID, l.service, nil)
		assert.Equal(t, http.StatusOK, gone, "учётная запись %s не удалена: %s", email, body)
	})
	return u
}

func TestLiveSupabaseFiles(t *testing.T) {
	l := liveSupabase{os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_ANON_KEY"), os.Getenv("SUPABASE_SERVICE_ROLE_KEY"), "",
		&http.Client{Timeout: 30 * time.Second}}
	region := os.Getenv("SUPABASE_REGION")
	if os.Getenv("GOPHKEEPER_LIVE") == "" || l.server == "" || l.app == "" || l.service == "" || region == "" {
		t.Skip("живой Supabase не задан")
	}
	rec := &recorder{next: http.DefaultTransport}
	web := &http.Client{Timeout: 5 * time.Minute, Transport: rec}
	as := func(token string) *storage.Client { return storage.New(web, l.server, region, l.app, token) }
	anna, boris := l.member(t, "anna"), l.member(t, "boris")
	owner, id, missing, data := anna.User.ID, uuid.New().String(), uuid.New().String(), []byte("шифр файла")
	admin := as(l.service)
	t.Cleanup(func() { admin.Delete(context.Background(), owner, id) })
	require.NoError(t, admin.Put(t.Context(), owner, id, data), "служебный ключ не записал объект")
	matchReference(t, "files_put.txt", rec.last(owner, id))
	got, err := admin.Get(t.Context(), owner, id, 1<<20)
	require.NoError(t, err, "служебный ключ не прочитал объект")
	assert.Equal(t, data, got, "служебный ключ прочитал не то")
	matchReference(t, "files_get.txt", rec.last(owner, id))
	_, err = as(anna.Access).Get(t.Context(), owner, id, 1<<20)
	require.NoError(t, err, "владелец не прочитал свой объект")
	assert.Equal(t, reference(t, "files_get.txt"), rec.last(owner, id), "выдача владельцу не равна выдаче служебным ключом")
	_, err = as(anna.Access).Get(t.Context(), owner, missing, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "отсутствующий объект не отмечен")
	matchReference(t, "files_missing.txt", rec.last(owner, missing))
	_, err = as(boris.Access).Get(t.Context(), owner, id, 1<<20)
	assert.ErrorIs(t, err, storage.ErrNoObject, "чужой объект не скрыт")
	assert.Equal(t, reference(t, "files_missing.txt"), rec.last(owner, id), "ответ на чужой объект не равен ответу на отсутствующий")
	assert.Error(t, as(anna.Access).Put(t.Context(), owner, id, []byte("подмена")), "пользователь записал объект сам")
	matchReference(t, "files_put_denied.txt", rec.last(owner, id))
	as(anna.Access).Delete(t.Context(), owner, id)
	matchReference(t, "files_delete_denied.txt", rec.last(owner, id))
	got, err = admin.Get(t.Context(), owner, id, 1<<20)
	assert.True(t, err == nil && string(got) == string(data), "отказы пользователю тронули объект: %v", err)
	assert.Error(t, as(spoil(anna.Access)).Delete(t.Context(), owner, id), "испорченный токен принят")
	matchReference(t, "files_bad_token.txt", rec.last(owner, id))
	as("").Delete(t.Context(), owner, id)
	matchReference(t, "files_no_token.txt", rec.last(owner, id))
	liveFileEdges(t, l, rec, admin, owner, id)
}

func liveFileEdges(t *testing.T, l liveSupabase, rec *recorder, admin *storage.Client, owner, id string) {
	t.Helper()
	assert.Error(t, rawObject(t, &http.Client{Transport: rec}, l.server, l.app, l.service, owner), "путь вне шаблона отдал объект")
	matchReference(t, "files_unknown.txt", rec.last(owner, id))
	edge, over := uuid.New().String(), uuid.New().String()
	t.Cleanup(func() { admin.Delete(context.Background(), owner, edge) })
	require.NoError(t, admin.Put(t.Context(), owner, edge, make([]byte, objectLimit)), "объект ровно на предел не принят")
	assert.Error(t, admin.Put(t.Context(), owner, over, make([]byte, objectLimit+1)), "объект больше предела принят")
	matchReference(t, "files_too_large.txt", rec.last(owner, over))
	require.NoError(t, admin.Delete(t.Context(), owner, id), "служебный ключ не удалил объект")
	matchReference(t, "files_delete.txt", rec.last(owner, id))
	require.NoError(t, admin.Delete(t.Context(), owner, id), "удаление отсутствующего объекта не успех")
	assert.Equal(t, reference(t, "files_delete.txt"), rec.last(owner, id), "удаление отсутствующего не равно удалению существующего")
}
