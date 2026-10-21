package cli

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func copyFiles(t *testing.T) map[string][]byte {
	t.Helper()
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	entries, err := os.ReadDir(filepath.Join(dir, "copy", annaID))
	require.NoError(t, err, "копии нет на устройстве")
	got := map[string][]byte{}
	for _, e := range entries {
		got[e.Name()], err = os.ReadFile(filepath.Join(dir, "copy", annaID, e.Name()))
		require.NoError(t, err, "файл копии не прочитан")
	}
	return got
}

func TestCopyKept(t *testing.T) {
	f := serveFile(t, 411, whole)
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	got := copyFiles(t)
	assert.Len(t, got, 2, "в копии не список и файл")
	assert.Equal(t, f.sealed, got[copyName(account.Entry{ID: f.rec.ID, Secret: f.rec.Secret})], "файл в копии не равен объекту")
	for name, b := range got {
		for _, open := range []string{"ключ SSH", "почта", "anna", "s3cret", "id_ed25519"} {
			assert.False(t, bytes.Contains(b, []byte(open)), "в %s открыто %q", name, open)
		}
	}
}

func TestCopyForgets(t *testing.T) {
	keep, gone := sealed(t, "anna", "s3cret", "почта"), sealed(t, "bob", "q", "банк")
	var second atomic.Bool
	srv := serveRecords(t, func(w http.ResponseWriter, r *http.Request) {
		recs := []stored{keep, gone}
		if second.Load() {
			recs = recs[:1]
		}
		replyJSON(t, recs)(w, r)
	})
	login(t)
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "первый список не получен")
	second.Store(true)
	_, _, err = executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "второй список не получен")
	srv.Close()
	out, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	assert.Contains(t, out, keep.ID, "оставшейся записи нет в копии")
	assert.NotContains(t, out, gone.ID, "удалённая запись осталась в копии")
}

func TestCopyStorageDown(t *testing.T) {
	f := serveFile(t, 411, func(_ *fileServer, _ int32, w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) })
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "сбой файлов уронил список")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"))
	assert.Equal(t, []string{copyList}, slices.Collect(maps.Keys(copyFiles(t))), "в копии файл без объекта")
	assert.NotEmpty(t, f.rec.ID, "нет записи о файле")
}

func TestCopyOffline(t *testing.T) {
	f := serveFile(t, 411, whole)
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	f.Close()
	out, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	one, err := noServer(t, "", "list", f.rec.ID)
	require.NoError(t, err, "запись без сервера не показана")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), one)
	assert.Contains(t, one, "сервер недоступен, показаны записи с устройства", "запись из копии без надписи")
	path := target(t)
	_, err = noServer(t, "", "get", f.rec.ID, path)
	require.NoError(t, err, "файл без сервера не сохранён")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "файл не записан")
	assert.Equal(t, f.plain, got, "файл из копии не равен исходному")
	_, err = noServer(t, "", "edit", f.rec.ID, "--note", "новая")
	assert.Error(t, err, "правка без сервера прошла")
}

func TestCopyOfflineUnsent(t *testing.T) {
	msg, id := unsentPut(t, annaData(t), map[string]string{"login": "anna", "password": "new"}, "новая")
	srv, q := queueDown(t, sealed(t, "bob", "q", "банк"))
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	onDevice(t, q, msg)
	srv.Close()
	out, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	assert.Contains(t, out, id, "неотправленной записи нет")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"))
}

func TestCopyRefresh(t *testing.T) {
	data := annaData(t)
	old := sealedAs(t, map[string]string{"type": "file", "name": "a.txt", "size": "3"}, "ключ")
	msg, err := record.Edit(data, annaID, old.ID, map[string]string{"type": "file", "name": "a.txt", "size": "3"}, "ключ")
	require.NoError(t, err, "правка не собрана")
	var c record.Change
	require.NoError(t, json.Unmarshal(msg.Data, &c), "тело правки не JSON")
	edited := stored{ID: old.ID, Note: old.Note, Secret: c.Secret}
	var second atomic.Bool
	files := map[bool][]byte{}
	for v, plain := range map[bool]string{false: "old", true: "new"} {
		files[v], err = record.SealFile(data, old.ID, []byte(plain))
		require.NoError(t, err, "файл не зашифрован")
	}
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/storage/v1/s3/"):
			_, _ = w.Write(files[second.Load()])
		case strings.HasPrefix(r.URL.Path, "/rest/v1/records"):
			replyJSON(t, []stored{map[bool]stored{false: old, true: edited}[second.Load()]})(w, r)
		default:
			ok(w, r)
		}
	})
	writeSettings(t, config.Settings{Server: srv.URL, AppKey: "ключ-приложения", Region: region, Queue: "tls://127.0.0.1:1", QueueLeaf: "tls://127.0.0.1:2"})
	login(t)
	_, _, err = executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "первый список не получен")
	second.Store(true)
	_, _, err = executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "второй список не получен")
	srv.Close()
	path := target(t)
	_, err = noServer(t, "", "get", old.ID, path)
	require.NoError(t, err, "файл без сервера не сохранён")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "файл не записан")
	assert.Equal(t, "new", string(got), "в копии осталась старая версия файла")
	assert.Len(t, copyFiles(t), 2, "старая версия файла не удалена из копии")
}

func TestCopyForeignUser(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "почта")
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/v1/records") {
			replyJSON(t, []stored{rec})(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "доступ", "refresh_token": "обновление", "user": map[string]string{"id": ".."}})
	})
	settingsFile(t, srv.URL, "ключ-приложения")
	login(t)
	_, _, _ = executeIn(keyboard(t, true, ""), "list")
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	assert.FileExists(t, filepath.Join(dir, config.File), "копия удалила файл настроек")
	assert.NoFileExists(t, filepath.Join(dir, copyList), "копия записана вне своего каталога")
}

func TestCopyDamaged(t *testing.T) {
	f := serveFile(t, 411, whole)
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	f.Close()
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	path := filepath.Join(dir, "copy", annaID, copyName(account.Entry{ID: f.rec.ID, Secret: f.rec.Secret}))
	require.NoError(t, os.Truncate(path, 100), "файл копии не обрезан")
	_, err = noServer(t, "", "get", f.rec.ID, target(t))
	assert.NotErrorIs(t, err, errDamaged, "порча копии выдана за порчу на сервере")
	assert.NoFileExists(t, path, "испорченный файл остался в копии")
}

func TestCopyUnwritable(t *testing.T) {
	f := serveFile(t, 411, whole)
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "copy"), 0o700), "каталог копий не создан")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "copy", annaID), nil, 0o600), "файл на месте копии не создан")
	out, errOut, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "сбой копии уронил список")
	assert.Contains(t, out, f.rec.ID, "записи нет в списке")
	assert.Contains(t, errOut, "копию записей на устройстве обновить не удалось", "нет предупреждения о копии")
}

func TestCopyUnreadable(t *testing.T) {
	msg, id := unsentPut(t, annaData(t), map[string]string{"login": "anna", "password": "new"}, "новая")
	kept := sealed(t, "bob", "q", "банк")
	srv, q := queueDown(t, kept)
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	onDevice(t, q, msg)
	srv.Close()
	dir, err := copyDir(annaID)
	require.NoError(t, err, "каталог копии не найден")
	path := filepath.Join(dir, copyList)
	require.NoError(t, os.Truncate(path, 10), "копия не испорчена")
	out, errOut, err := noServerOut(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	assert.Contains(t, out, id, "неотправленной записи нет")
	one, oneErr, err := noServerOut(t, "", "list", kept.ID)
	assert.Equal(t, account.ErrUnavailable, err, "нет «сервер недоступен» для записи")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), errOut, one, oneErr)
	require.NoError(t, os.Remove(path), "копия не удалена")
	_, errOut, err = noServerOut(t, "", "list")
	require.NoError(t, err, "список без копии не показан")
	assert.Empty(t, errOut, "надпись о копии без копии")
}
