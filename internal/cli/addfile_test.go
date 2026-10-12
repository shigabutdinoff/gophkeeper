package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const region = "eu-north-1"

func withRegion(s config.Settings) config.Settings {
	s.Region = region
	return s
}

func picking(t *testing.T, name string, size int64) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	data := make([]byte, min(size, 1<<10))
	rand.Read(data)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, data, 0o600), "файл не записан")
	require.NoError(t, os.Truncate(path, size), "размер файла не задан")
	typing(t, "")
	return path, data
}

func waitingFile(t *testing.T, q *queuetest.Queue, js jetstream.JetStream, id string) (*jetstream.ObjectInfo, []byte) {
	t.Helper()
	obs, err := js.ObjectStore(t.Context(), record.Bucket(q.Seed, annaID))
	require.NoError(t, err, "корзина владельца не открыта")
	info, err := obs.GetInfo(t.Context(), id)
	require.NoError(t, err, "файла нет в корзине")
	sealed, err := obs.GetBytes(t.Context(), id)
	require.NoError(t, err, "файл не читается из корзины")
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	plain, err := record.OpenFile(data, id, sealed)
	require.NoError(t, err, "файл из корзины не расшифрован")
	return info, plain
}

func TestAddFile(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	_, data := picking(t, "id_ed25519", 411)
	out, errOut, err := executeIn(input(t, ""), "add", "--type", "file", "--note", "ключ SSH для рабочего сервера")
	require.NoError(t, err, "файл не принят")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), errOut)
	c := lastChange(t, stream)
	assert.Equal(t, []string{"put", "ключ SSH для рабочего сервера"}, []string{c.Op, c.Note}, "в очереди не та запись")
	assert.Contains(t, out, c.Record, "напечатан не тот идентификатор записи")
	assert.Equal(t, map[string]string{"type": "file", "name": "id_ed25519", "size": "411"}, secretOf(t, c), "не те имя и размер файла")
	info, plain := waitingFile(t, q, js, c.Record)
	assert.Equal(t, info.Digest, c.Digest, "сумма в записи не равна сумме объекта")
	assert.Equal(t, data, plain[:len(data)], "в корзине не тот файл")
	assert.Len(t, plain, 411, "в корзине файл не того размера")
	assert.Equal(t, uint64(1), queued(t, stream), "в очереди не одно изменение")
}

func TestAddFileOffline(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	srv := loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	srv.Close()
	picking(t, "скан.pdf", 0)
	out, _, err := executeIn(input(t, ""), "add", "--type", "file")
	require.NoError(t, err, "файл без сервера не принят")
	c := lastChange(t, stream)
	assert.Contains(t, out, c.Record, "напечатан не тот идентификатор записи")
	assert.Equal(t, map[string]string{"type": "file", "name": "скан.pdf", "size": "0"}, secretOf(t, c), "не те имя и размер пустого файла")
	_, plain := waitingFile(t, q, js, c.Record)
	assert.Empty(t, plain, "пустой файл стал непустым")
}

func TestAddFileLimit(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	picking(t, "huge.bin", record.MaxFile+1)
	out, _, err := executeIn(input(t, ""), "add", "--type", "file")
	require.Error(t, err, "файл больше предела принят")
	assert.Empty(t, out, "при ошибке напечатано подтверждение")
	snaps.MatchSnapshot(t, err.Error())
	_, err = js.ObjectStore(t.Context(), record.Bucket(q.Seed, annaID))
	assert.ErrorIs(t, err, jetstream.ErrBucketNotFound, "для файла больше предела открыта корзина")
	assert.Zero(t, queued(t, stream), "запись о файле больше предела ушла в очередь")

	picking(t, "big.bin", record.MaxFile)
	_, _, err = executeIn(input(t, ""), "add", "--type", "file")
	require.NoError(t, err, "файл ровно в предел не принят")
	_, plain := waitingFile(t, q, js, lastChange(t, stream).Record)
	assert.Len(t, plain, record.MaxFile, "файл в предел дошёл не целиком")
}

func TestAddFileRefuses(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	path, _ := picking(t, "id_ed25519", 411)
	form := runForm
	setVar(t, &runForm, func(c *cobra.Command, f *huh.Form) error {
		defer os.Remove(path)
		return form(c, f)
	})
	_, _, err := executeIn(input(t, ""), "add", "--type", "file")
	require.EqualError(t, err, "файл id_ed25519 не найден", "пропавший файл принят")

	keyboard(t, false)
	setVar(t, &runForm, func(*cobra.Command, *huh.Form) error { return errors.New("форма открыта") })
	_, _, err = executeIn(input(t, ""), "add", "--type", "file")
	require.Error(t, err, "файл принят без терминала")
	_, _, flagErr := executeIn(input(t, ""), "add", "--type", "file", "--file", "x")
	require.Error(t, flagErr, "флаг --file принят")
	loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	_, _, regionErr := executeIn(input(t, ""), "add", "--type", "file")
	require.Error(t, regionErr, "файл принят без региона в настройках")
	snaps.MatchSnapshot(t, err.Error(), flagErr.Error(), regionErr.Error())
	assert.Zero(t, queued(t, stream), "при отказе запись ушла в очередь")
}

func TestAddFileQueueRefuses(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Run(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	picking(t, "id_ed25519", 411)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	out, _, err := executeCtx(ctx, input(t, ""), "add", "--type", "file")
	require.ErrorIs(t, err, queue.ErrUnavailable, "отказ очереди на файле не распознан")
	assert.Empty(t, out, "при ошибке напечатано подтверждение")
	assert.Zero(t, queued(t, stream), "запись о файле ушла в очередь без файла")
}

func TestAddFailedPromptSkipsFile(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	picking(t, "id_ed25519", 411)
	out, _, err := executeIn(input(t, ""), "add", "--login", "anna")
	require.EqualError(t, err, "пароль записи не может быть пустым", "пустой пароль записи принят")
	assert.Empty(t, out, "после ошибки ввода напечатано подтверждение")
	assert.Zero(t, queued(t, stream), "после ошибки ввода в очередь ушла запись о файле")
	_, err = js.ObjectStore(t.Context(), record.Bucket(q.Seed, annaID))
	assert.ErrorIs(t, err, jetstream.ErrBucketNotFound, "после ошибки ввода открыта корзина файлов")
}

func TestAddFileRejectsRecordBeforeUpload(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	picking(t, "id_ed25519", 411)
	_, _, err := executeIn(input(t, ""), "add", "--type", "file", "--note", strings.Repeat("я", 40000))
	require.ErrorIs(t, err, record.ErrTooLarge, "запись больше 64 КиБ принята")
	assert.Zero(t, queued(t, stream), "запись больше 64 КиБ ушла в очередь")
	obs, err := js.ObjectStore(t.Context(), record.Bucket(q.Seed, annaID))
	if err == nil {
		_, err = obs.List(t.Context())
		assert.ErrorIs(t, err, jetstream.ErrNoObjectsFound, "файл без записи остался в корзине")
	}
}
