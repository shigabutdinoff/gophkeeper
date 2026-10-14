package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func savedOnDevice(t *testing.T) (*queuetest.Queue, string) {
	t.Helper()
	q := queuetest.New(t)
	srv := serveAs(t, q.Token(t, annaID, time.Hour), replyJSON(t, []stored{}))
	queueSettings(t, srv.URL, q.Client)
	login(t)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	out, _, err := executeCtx(ctx, keyboard(t, true, "s3cret"), "add", "--login", "anna", "--note", "сайт example.com")
	require.NoError(t, err, "запись без очереди не сохранена на устройстве")
	return q, out
}

func TestAddQueueDown(t *testing.T) {
	_, out := savedOnDevice(t)
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"))
	assert.True(t, queue.Kept(), "запись не осталась на устройстве")
}

func TestAddSentLater(t *testing.T) {
	q, out := savedOnDevice(t)
	_, stream := q.Start(t, false)
	for range 2 {
		_, _, err := executeIn(keyboard(t, true), "list")
		require.NoError(t, err, "список не получен")
		assert.Equal(t, uint64(1), queued(t, stream), "в очереди не одно изменение")
	}
	assert.Contains(t, out, lastChange(t, stream).Record, "в очереди не та запись")
	assert.False(t, queue.Kept(), "отправленная запись осталась на устройстве")
}

func TestAddKeptEncrypted(t *testing.T) {
	savedOnDevice(t)
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	var size int
	err = filepath.WalkDir(filepath.Join(dir, "outbox"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		size += len(data)
		for _, open := range []string{"s3cret", "anna", "example.com", annaID} {
			assert.False(t, bytes.Contains(data, []byte(open)), "в %s открыто %q", path, open)
		}
		return err
	})
	require.NoError(t, err, "очередь устройства не прочитана")
	assert.Positive(t, size, "очередь устройства пуста")
}

func TestLogoutKept(t *testing.T) {
	savedOnDevice(t)
	out, errOut, err := executeIn(input(t, ""), "logout")
	require.NoError(t, err, "выход не выполнен")
	snaps.MatchSnapshot(t, out, errOut)
	assert.True(t, queue.Kept(), "неотправленная запись удалена при выходе")
}
