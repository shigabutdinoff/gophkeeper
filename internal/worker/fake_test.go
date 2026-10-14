package worker

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

const s3Path = "/storage/v1/s3/files/"

type call struct {
	Method string `json:"-"`
	Table  string `json:"-"`
	Owner  string `json:"user_id"`
	Record string `json:"id"`
	Note   string `json:"note"`
	Secret string `json:"secret"`
	APIKey string `json:"-"`
	Auth   string `json:"-"`
}

type fake struct {
	*httptest.Server
	mu    sync.Mutex
	rows  map[[2]string]string
	calls []call
	reply func(n int, w http.ResponseWriter) bool
	log   syncBuffer

	objects map[string]string
	trace   []string
	files   func(n int, w http.ResponseWriter) bool
}

func (f *fake) serveS3(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, s3Path)
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	n, files := len(f.trace), f.files
	f.trace = append(f.trace, r.Method+" "+key+" "+r.Header.Get("X-Amz-Security-Token"))
	f.mu.Unlock()
	if files != nil && files(n, w) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		f.objects[key] = string(body)
	case http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (f *fake) stored() ([]string, map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	objects := map[string]string{}
	for k, v := range f.objects {
		objects[k] = v
	}
	return append([]string(nil), f.trace...), objects
}

func newFake(t *testing.T) *fake {
	f := &fake{rows: map[[2]string]string{}, objects: map[string]string{}}
	f.Server = httptest.NewTLSServer(f)
	t.Cleanup(f.Close)
	return f
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, s3Path) {
		f.serveS3(w, r)
		return
	}
	q := r.URL.Query()
	c := call{Method: r.Method, Table: strings.TrimPrefix(r.URL.Path, "/rest/v1/"),
		APIKey: r.Header.Get("apikey"), Auth: r.Header.Get("Authorization")}
	body := r.Method == http.MethodPost || r.Method == http.MethodPatch
	switch {
	case q.Get("select") != "user_id" || !strings.Contains(r.Header.Get("Prefer"), "return=representation"),
		body && json.NewDecoder(r.Body).Decode(&c) != nil,
		r.Method == http.MethodPost && q.Get("on_conflict") != "user_id,id":
		http.Error(w, "неверный запрос", http.StatusBadRequest)
		return
	case r.Method != http.MethodPost && c.Table == "records":
		c.Owner, c.Record = strings.TrimPrefix(q.Get("user_id"), "eq."), strings.TrimPrefix(q.Get("id"), "eq.")
	}
	f.mu.Lock()
	n, reply := len(f.calls), f.reply
	f.calls = append(f.calls, c)
	f.trace = append(f.trace, r.Method+" "+c.Table)
	f.mu.Unlock()
	if reply != nil && reply(n, w) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	row := [2]string{c.Owner, c.Record}
	_, exists := f.rows[row]
	found := `[{"user_id":"` + c.Owner + `"}]`
	switch {
	case c.Table == "records" && r.Method == http.MethodDelete && exists:
		delete(f.rows, row)
	case c.Table == "records" && (r.Method == http.MethodPost || r.Method == http.MethodPatch && exists):
		f.rows[row] = c.Note + "/" + c.Secret
	default:
		found = "[]"
	}
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusCreated)
	}
	_, _ = w.Write([]byte(found))
}

func (f *fake) writes() []call {
	calls, _ := f.snapshot()
	return calls
}

func (f *fake) server(tune ...tuning) Server {
	t := tuning{}
	if len(tune) > 0 {
		t = tune[0]
	}
	t.maxDelay, t.heartbeat = 10*time.Millisecond, 100*time.Millisecond
	return Server{URL: f.URL, Key: "ключ-сервера", Client: f.Client(), Log: log.New(&f.log, "", 0),
		Files: storage.New(f.Client(), f.URL, "eu-north-1", "ключ-приложения", "служебный-ключ"), Seed: seed, tuning: t}
}

func (f *fake) snapshot() ([]call, map[[2]string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := map[[2]string]string{}
	for k, v := range f.rows {
		rows[k] = v
	}
	return append([]call(nil), f.calls...), rows
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) lines(substr string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var got []string
	for _, l := range strings.Split(b.buf.String(), "\n") {
		if l != "" && strings.Contains(l, substr) {
			got = append(got, l)
		}
	}
	return got
}
