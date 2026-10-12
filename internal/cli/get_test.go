package cli

import (
	"crypto/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

type fileServer struct {
	*signupServer
	rec      stored
	plain    []byte
	sealed   []byte
	objects  atomic.Int32
	path     string
	token    string
	settings config.Settings
}

func serveFile(t *testing.T, size int, object func(f *fileServer, n int32, w http.ResponseWriter)) *fileServer {
	t.Helper()
	f := &fileServer{plain: make([]byte, 411)}
	rand.Read(f.plain)
	f.rec = sealedAs(t, map[string]string{"type": "file", "name": "id_ed25519", "size": strconv.Itoa(size)}, "ключ SSH для рабочего сервера")
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	f.sealed, err = record.SealFile(data, f.rec.ID, f.plain)
	require.NoError(t, err, "файл не зашифрован")
	records := replyJSON(t, []stored{f.rec, sealed(t, "anna", "s3cret", "почта")})
	f.signupServer = startServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/storage/v1/s3/"):
			f.path, f.token = r.URL.Path, r.Header.Get("X-Amz-Security-Token")
			object(f, f.objects.Add(1), w)
		case strings.HasPrefix(r.URL.Path, "/rest/v1/records"):
			records(w, r)
		default:
			ok(w, r)
		}
	})
	f.settings = config.Settings{Server: f.URL, AppKey: "ключ-приложения", Region: region, Queue: "tls://127.0.0.1:1"}
	writeSettings(t, f.settings)
	login(t)
	return f
}

func whole(f *fileServer, _ int32, w http.ResponseWriter) {
	w.Write(f.sealed)
}

func target(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "key")
}

func TestListFile(t *testing.T) {
	f := serveFile(t, 411, whole)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список с файлом не получен")
	one, _, err := executeIn(keyboard(t, true, ""), "list", f.rec.ID)
	require.NoError(t, err, "запись о файле не получена")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), one)
	assert.Zero(t, f.objects.Load(), "список читал содержимое файла")
}

func TestGet(t *testing.T) {
	f := serveFile(t, 411, whole)
	path := target(t)
	out, errOut, err := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
	require.NoError(t, err, "файл не получен")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "файл не записан")
	assert.Equal(t, f.plain, got, "файл не равен исходному")
	info, err := os.Stat(path)
	require.NoError(t, err, "файл не найден")
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "файл доступен другим")
	assert.Equal(t, []string{"/storage/v1/s3/files/" + annaID + "/" + f.rec.ID, "доступ"}, []string{f.path, f.token}, "не тот запрос файла")
	snaps.MatchSnapshot(t, strings.ReplaceAll(out, path, "<путь>"), errOut)

	keyring.MockInit()
	writeSettings(t, f.settings)
	login(t)
	other := target(t)
	_, _, err = executeIn(keyboard(t, true, ""), "get", f.rec.ID, other)
	require.NoError(t, err, "файл не получен на втором устройстве")
	got, err = os.ReadFile(other)
	require.NoError(t, err, "файл на втором устройстве не записан")
	assert.Equal(t, f.plain, got, "на втором устройстве не тот файл")
}

func TestGetSilent(t *testing.T) {
	release := make(chan struct{})
	f := serveFile(t, 411, func(*fileServer, int32, http.ResponseWriter) { <-release })
	t.Cleanup(func() { close(release) })
	path, start := target(t), time.Now()
	_, _, err := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
	assert.Equal(t, account.ErrNoAnswer, err, "нет «сервер не отвечает»")
	assert.Less(t, time.Since(start), 2*time.Second, "команда ждала дольше срока")
	assert.EqualValues(t, 1, f.objects.Load(), "молчащий сервер опрошен повторно")
	assert.NoFileExists(t, path, "при молчании сервера появился файл")
}

func TestGetResumes(t *testing.T) {
	f := serveFile(t, 411, func(f *fileServer, n int32, w http.ResponseWriter) {
		if n > 1 {
			w.Write(f.sealed)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(f.sealed)))
		w.Write(f.sealed[:200])
	})
	path := target(t)
	_, _, err := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
	require.NoError(t, err, "обрыв не пережит")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "файл не записан")
	assert.Equal(t, f.plain, got, "после обрыва файл не равен исходному")
	assert.EqualValues(t, 2, f.objects.Load(), "файл получен не со второй попытки")
}

func TestGetDamaged(t *testing.T) {
	cases := map[string]struct {
		size   int
		object func(f *fileServer, n int32, w http.ResponseWriter)
	}{
		"объекта нет": {411, func(_ *fileServer, _ int32, w http.ResponseWriter) {
			http.Error(w, "<Error><Code>NoSuchKey</Code></Error>", http.StatusNotFound)
		}},
		"объект обрезан": {411, func(f *fileServer, _ int32, w http.ResponseWriter) { w.Write(f.sealed[:300]) }},
		"размер не тот":  {5, whole},
	}
	var errs []string
	for name, c := range cases {
		f := serveFile(t, c.size, c.object)
		path := target(t)
		_, _, err := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
		require.Error(t, err, "повреждённый файл принят: %s", name)
		errs = append(errs, err.Error())
		assert.NoFileExists(t, path, "появился повреждённый файл: %s", name)
		assert.EqualValues(t, 1, f.objects.Load(), "повреждённый файл запрошен повторно: %s", name)
	}
	assert.Equal(t, []string{errs[0], errs[0], errs[0]}, errs, "ошибки о повреждённом файле разные")
	snaps.MatchSnapshot(t, errs[0])
}

func TestGetRefuses(t *testing.T) {
	f := serveFile(t, 411, whole)
	path := target(t)
	require.NoError(t, os.WriteFile(path, []byte("прежний файл"), 0o600), "прежний файл не записан")
	before := f.requests.Load()
	_, _, taken := executeIn(keyboard(t, true, ""), "get", f.rec.ID, path)
	require.Error(t, taken, "занятый путь принят")
	assert.Equal(t, before, f.requests.Load(), "сервер опрошен до проверки пути")
	got, err := os.ReadFile(path)
	require.NoError(t, err, "прежний файл пропал")
	assert.Equal(t, "прежний файл", string(got), "прежний файл изменён")

	free := target(t)
	var errs []string
	for _, args := range [][]string{{"abc", free}, {}, {f.rec.ID}, {f.rec.ID, free, "x"}} {
		_, _, err := executeIn(keyboard(t, true, ""), append([]string{"get"}, args...)...)
		require.Error(t, err, "неверные аргументы приняты: %q", args)
		errs = append(errs, err.Error())
	}
	f.settings.Region = ""
	writeSettings(t, f.settings)
	login(t)
	_, _, noRegion := executeIn(keyboard(t, true, ""), "get", f.rec.ID, free)
	require.Error(t, noRegion, "файл получен без региона")
	_, _, foreign := executeIn(keyboard(t, true, ""), "get", sealed(t, "anna", "s3cret", "почта").ID, free)
	assert.ErrorIs(t, foreign, account.ErrNotFound, "чужая запись найдена")
	snaps.MatchSnapshot(t, strings.ReplaceAll(taken.Error(), path, "<путь>"), errs, noRegion.Error())
	assert.NoFileExists(t, free, "при отказе появился файл")
}

func TestGetOtherKind(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "почта")
	srv := serveRecords(t, replyJSON(t, []stored{rec}))
	queueSettings(t, srv.URL, config.Settings{Queue: "tls://127.0.0.1:1"})
	login(t)
	path := target(t)
	_, _, err := executeIn(keyboard(t, true, ""), "get", rec.ID, path)
	require.Error(t, err, "логин получен как файл")
	snaps.MatchSnapshot(t, strings.ReplaceAll(err.Error(), rec.ID, "<id>"))
	assert.NoFileExists(t, path, "для логина появился файл")
}
