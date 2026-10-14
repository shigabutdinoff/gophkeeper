package server

import (
	"errors"
	"net/http"
	"testing"
	"uuid"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

func (r *running) files(token string) *storage.Client {
	return storage.New(r.client(), "https://"+r.addr, "us-east-1", "app-key", token)
}

func TestFilesDeleteConfirmed(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	require.NoError(t, r.files("service-key").Delete(t.Context(), anna.User.ID, id), "удаление файла служебным ключом не подтверждено")
	err := r.files("app-key").Delete(t.Context(), anna.User.ID, id)
	re, ok := errors.AsType[*awshttp.ResponseError](err)
	require.True(t, ok, "удаление файла чужим токеном не отклонено: %v", err)
	assert.Equal(t, http.StatusForbidden, re.HTTPStatusCode(), "отказ в удалении файла не 403")
	assert.Error(t, r.files("service-key").Put(t.Context(), anna.User.ID, id, []byte("файл")), "файл сохранён на своём сервере до GOP-9")
	status, _, _ := r.send(t, http.MethodDelete, "/storage/v1/s3/files/"+anna.User.ID+"/"+id, "", nil)
	assert.Equal(t, http.StatusForbidden, status, "удаление файла без токена сессии не отклонено")
	status, _, _ = r.send(t, http.MethodGet, "/storage/v1/s3/files/"+anna.User.ID+"/"+id, "", nil)
	assert.Equal(t, http.StatusUnauthorized, status, "чтение файла без ключа проекта не отклонено")
	kept, ok := r.saved(t, anna.User.ID, id)
	assert.True(t, ok && kept.note == "почта", "подтверждение удаления файла тронуло запись")
}

func TestFilesNeedRegion(t *testing.T) {
	r := start(t)
	err := storage.New(r.client(), "https://"+r.addr, "", "app-key", "service-key").Delete(t.Context(), uuid.New().String(), uuid.New().String())
	_, answered := errors.AsType[*awshttp.ResponseError](err)
	assert.True(t, err != nil && !answered, "клиент хранилища без региона обратился к серверу: %v", err)
}
