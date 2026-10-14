package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	anna   = "0199b5e4-0000-7000-8000-00000000a11a"
	record = "0199b5e4-7c3a-7d21-9f4e-2a6c8b1d3e57"
	path   = "/storage/v1/s3/files/" + anna + "/" + record
)

type fakeS3 struct {
	*httptest.Server
	requests atomic.Int32
	client   *Client
}

func startS3(t *testing.T, reply http.HandlerFunc) *fakeS3 {
	t.Helper()
	f := &fakeS3{}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		reply(w, r)
	}))
	t.Cleanup(f.Close)
	web := f.Server.Client()
	web.Timeout = 300 * time.Millisecond
	f.client = New(web, f.URL+"/", "eu-north-1", "ключ-приложения", "токен")
	return f
}

func TestPut(t *testing.T) {
	var method, target, token, auth, body string
	f := startS3(t, func(_ http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		method, target, body = r.Method, r.URL.Path, string(data)
		token, auth = r.Header.Get("X-Amz-Security-Token"), r.Header.Get("Authorization")
	})
	require.NoError(t, f.client.Put(t.Context(), anna, record, []byte("шифр")), "объект не записан")
	assert.Equal(t, []string{http.MethodPut, path, "шифр", "токен"}, []string{method, target, body, token}, "не тот запрос записи")
	assert.Regexp(t, `Credential=127/\d{8}/eu-north-1/s3/aws4_request`, auth, "в подписи нет проекта и региона")
	assert.EqualValues(t, 1, f.requests.Load(), "запрос записи повторён")
}

func TestGet(t *testing.T) {
	var method, target string
	f := startS3(t, func(w http.ResponseWriter, r *http.Request) {
		method, target = r.Method, r.URL.Path
		_, _ = w.Write([]byte("шифр файла"))
	})
	data, err := f.client.Get(t.Context(), anna, record, 100)
	require.NoError(t, err, "объект не прочитан")
	assert.Equal(t, []string{http.MethodGet, path, "шифр файла"}, []string{method, target, string(data)}, "не тот запрос чтения")
	_, err = f.client.Get(t.Context(), anna, record, 5)
	assert.Error(t, err, "объект больше предела принят")
}

func TestMissingObject(t *testing.T) {
	for name, body := range map[string]string{"NoSuchKey": "<Error><Code>NoSuchKey</Code></Error>", "404": ""} {
		f := startS3(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, body, http.StatusNotFound)
		})
		_, err := f.client.Get(t.Context(), anna, record, 100)
		assert.ErrorIs(t, err, ErrNoObject, "ответ %s не дал «объекта нет»", name)
		assert.NoError(t, f.client.Delete(t.Context(), anna, record), "удаление отсутствующего объекта не успех: %s", name)
	}
}

func TestDelete(t *testing.T) {
	var method, target string
	f := startS3(t, func(w http.ResponseWriter, r *http.Request) {
		method, target = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, f.client.Delete(t.Context(), anna, record), "объект не удалён")
	assert.Equal(t, []string{http.MethodDelete, path}, []string{method, target}, "не тот запрос удаления")
}

func TestSingleAttempt(t *testing.T) {
	replies := map[string]http.HandlerFunc{
		"сервер держит ответ": func(_ http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
		},
		"ответ 503": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
	}
	for name, reply := range replies {
		f := startS3(t, reply)
		_, err := f.client.Get(t.Context(), anna, record, 100)
		require.Error(t, err, "нет ошибки: %s", name)
		assert.NotErrorIs(t, err, ErrNoObject, "сбой принят за отсутствие объекта: %s", name)
		assert.Error(t, f.client.Put(t.Context(), anna, record, []byte("шифр")), "запись прошла: %s", name)
		assert.EqualValues(t, 2, f.requests.Load(), "запрос повторён: %s", name)
	}
}
