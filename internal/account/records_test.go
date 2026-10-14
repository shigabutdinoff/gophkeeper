package account

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const annaRecord = "0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d"

type postgrest struct {
	*httptest.Server
	requests atomic.Int32
	settings config.Settings
	client   *http.Client
}

func startPostgrest(t *testing.T, reply func(n int32, w http.ResponseWriter, r *http.Request)) *postgrest {
	t.Helper()
	p := &postgrest{}
	p.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply(p.requests.Add(1), w, r)
	}))
	t.Cleanup(p.Close)
	p.client = p.Client()
	p.client.Timeout = 2 * time.Second
	p.settings = config.Settings{Server: p.URL, AppKey: "ключ"}
	return p
}

func records(body string) func(int32, http.ResponseWriter, *http.Request) {
	return func(_ int32, w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }
}

func TestList(t *testing.T) {
	var query, apikey, bearer string
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, r *http.Request) {
		query, apikey, bearer = r.URL.Path+"?"+r.URL.RawQuery, r.Header.Get("apikey"), r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[{"id":"1","note":"почта"},{"id":"2","note":""},{"id":"3","note":"банк"}]`))
	})
	list, err := List(t.Context(), p.client, p.settings, "доступ")
	require.NoError(t, err, "список не получен")
	assert.Equal(t, []Entry{{ID: "1", Note: "почта"}, {ID: "2"}, {ID: "3", Note: "банк"}}, list, "не тот список")
	assert.Equal(t, "/rest/v1/records?select=id,note,secret&order=id&limit=100", query, "не тот запрос списка")
	assert.Equal(t, "ключ", apikey, "нет ключа приложения")
	assert.Equal(t, "Bearer доступ", bearer, "нет токена доступа")
}

func TestFind(t *testing.T) {
	var query string
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"id":"` + annaRecord + `","note":"почта","secret":"c2VrcmV0"}]`))
	})
	e, err := Find(t.Context(), p.client, p.settings, "доступ", annaRecord)
	require.NoError(t, err, "запись не получена")
	assert.Equal(t, Entry{ID: annaRecord, Note: "почта", Secret: []byte("sekret")}, e, "не та запись")
	assert.Equal(t, "select=id,note,secret&id=eq."+annaRecord, query, "не тот запрос записи")
}

func TestFindNotFound(t *testing.T) {
	p := startPostgrest(t, records(`[]`))
	_, err := Find(t.Context(), p.client, p.settings, "доступ", annaRecord)
	assert.ErrorIs(t, err, ErrNotFound, "пустой ответ не дал «запись не найдена»")
	_, err = Find(t.Context(), p.client, p.settings, "доступ", "abc")
	assert.ErrorIs(t, err, ErrNotFound, "неверный идентификатор не дал «запись не найдена»")
	assert.Equal(t, int32(1), p.requests.Load(), "неверный идентификатор ушёл на сервер")
}

func TestListRetriesBusyServer(t *testing.T) {
	for _, status := range []int{429, 502, 503, 504} {
		p := startPostgrest(t, func(n int32, w http.ResponseWriter, _ *http.Request) {
			if n == 1 {
				w.WriteHeader(status)
				return
			}
			_, _ = w.Write([]byte(`[{"id":"1"}]`))
		})
		list, err := List(t.Context(), p.client, p.settings, "доступ")
		require.NoError(t, err, "ответ %d не повторён", status)
		assert.Len(t, list, 1, "после повтора нет записей")
		assert.Equal(t, int32(2), p.requests.Load(), "после ответа %d не два запроса", status)
	}
}

func TestListBusyForever(t *testing.T) {
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) })
	_, err := List(t.Context(), p.client, p.settings, "доступ")
	assert.ErrorIs(t, err, ErrServer, "постоянная перегрузка не дала ошибку сервера")
	assert.Equal(t, int32(4), p.requests.Load(), "не четыре попытки")
}

func TestListHidesServerDetails(t *testing.T) {
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"secret-detail","message":"secret-detail"}`))
	})
	_, err := List(t.Context(), p.client, p.settings, "доступ")
	require.ErrorIs(t, err, ErrServer, "незнакомая ошибка не дала ошибку сервера")
	assert.NotContains(t, err.Error(), "secret-detail", "текст сервера попал в ошибку")
	assert.Equal(t, int32(1), p.requests.Load(), "ответ 500 повторён")
}

func TestReasonUnavailable(t *testing.T) {
	p := startPostgrest(t, records(`[]`))
	p.Close()
	_, err := List(t.Context(), p.client, p.settings, "доступ")
	assert.Equal(t, ErrUnavailable, Reason(err), "отказ соединения не дал «сервер недоступен»")
}

func TestReasonSilentServer(t *testing.T) {
	release := make(chan struct{})
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, _ *http.Request) { <-release })
	defer close(release)
	ctx, cancel := context.WithCancelCause(t.Context())
	time.AfterFunc(50*time.Millisecond, func() { cancel(context.DeadlineExceeded) })
	_, err := List(ctx, p.client, p.settings, "доступ")
	assert.Equal(t, ErrNoAnswer, Reason(err), "молчащий сервер не дал «сервер не отвечает»")
	assert.Equal(t, int32(1), p.requests.Load(), "таймаут повторён")
}

func TestReasonKeepsOtherErrors(t *testing.T) {
	assert.Equal(t, ErrServer, Reason(ErrServer), "ошибка сервера изменена")
	assert.Nil(t, Reason(nil), "пустая ошибка изменена")
}

func TestRetryWritesNothing(t *testing.T) {
	p := startPostgrest(t, records(`[]`))
	p.Close()
	r, w, err := os.Pipe()
	require.NoError(t, err, "канал не создан")
	stderr := os.Stderr
	os.Stderr = w
	_, _ = List(t.Context(), p.client, p.settings, "доступ")
	os.Stderr = stderr
	require.NoError(t, w.Close(), "канал не закрыт")
	out, err := io.ReadAll(r)
	require.NoError(t, err, "канал не прочитан")
	assert.Empty(t, string(out), "повтор писал в stderr")
}

func TestPostDoesNotRetry(t *testing.T) {
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) })
	err := post(t.Context(), p.client, p.settings, "auth/v1/signup", "", struct{}{}, nil)
	assert.ErrorIs(t, err, ErrServer, "перегрузка не дала ошибку сервера")
	assert.Equal(t, int32(1), p.requests.Load(), "регистрация повторена")
}

func paged(t *testing.T, rows []Entry, max int) func(int32, http.ResponseWriter, *http.Request) {
	return func(_ int32, w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if after, ok := strings.CutPrefix(r.URL.Query().Get("id"), "gt."); ok {
			offset = slices.IndexFunc(rows, func(e Entry) bool { return e.ID == after }) + 1
		}
		if limit == 0 || limit > max {
			limit = max
		}
		page := rows[min(offset, len(rows)):min(offset+limit, len(rows))]
		require.NoError(t, json.NewEncoder(w).Encode(page), "страница не записана")
	}
}

func TestListAllPages(t *testing.T) {
	rows := make([]Entry, 2345)
	for i := range rows {
		rows[i] = Entry{ID: strconv.Itoa(i), Note: "пометка"}
	}
	p := startPostgrest(t, paged(t, rows, 1000))
	list, err := List(t.Context(), p.client, p.settings, "доступ")
	require.NoError(t, err, "список не получен")
	assert.Equal(t, rows, list, "получены не все записи")
}

func TestListDeleteBetweenPages(t *testing.T) {
	rows := make([]Entry, 150)
	for i := range rows {
		rows[i] = Entry{ID: strconv.Itoa(i), Note: "пометка"}
	}
	p := startPostgrest(t, func(n int32, w http.ResponseWriter, r *http.Request) {
		paged(t, rows[min(n-1, 1):], 1000)(n, w, r)
	})
	list, err := List(t.Context(), p.client, p.settings, "доступ")
	require.NoError(t, err, "список не получен")
	assert.Equal(t, rows, list, "удаление между страницами скрыло запись")
}

func TestListLargeNotes(t *testing.T) {
	rows := make([]Entry, 50)
	for i := range rows {
		rows[i] = Entry{ID: strconv.Itoa(i), Note: strings.Repeat("я", 30000)}
	}
	p := startPostgrest(t, paged(t, rows, 1000))
	list, err := List(t.Context(), p.client, p.settings, "доступ")
	require.NoError(t, err, "большой список не получен")
	assert.Len(t, list, 50, "получены не все записи")
}

func TestFindLargeRecord(t *testing.T) {
	note := strings.Repeat("я", 40000)
	p := startPostgrest(t, func(_ int32, w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode([]Entry{{ID: annaRecord, Note: note, Secret: []byte("sekret")}}), "ответ не записан")
	})
	e, err := Find(t.Context(), p.client, p.settings, "доступ", annaRecord)
	require.NoError(t, err, "большая запись не получена")
	assert.Equal(t, note, e.Note, "пометка обрезана")
}
