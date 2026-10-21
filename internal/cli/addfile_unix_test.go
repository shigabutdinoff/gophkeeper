//go:build unix

package cli

import (
	"os"
	"syscall"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func TestAddFileGrownPastLimit(t *testing.T) {
	q := queuetest.New(t)
	js, stream := q.Start(t, false)
	loggedIn(t, withRegion(q.Client), q.Token(t, annaID, time.Hour))
	path, _ := picking(t, "grow.log", 411)
	form := runForm
	setVar(t, &runForm, func(c *cobra.Command, f *huh.Form) error {
		err := form(c, f)
		require.NoError(t, os.Remove(path), "выбранный файл не убран")
		require.NoError(t, syscall.Mkfifo(path, 0o600), "растущий файл не создан")
		go func() {
			if w, werr := os.OpenFile(path, os.O_WRONLY, 0); werr == nil {
				_, _ = w.Write(make([]byte, record.MaxFile+1))
				_ = w.Close()
			}
		}()
		return err
	})
	out, _, err := executeIn(input(t, ""), "add", "--type", "file")
	require.EqualError(t, err, "файлы больше 50 МБ не поддерживаются", "файл, выросший за предел после выбора, принят")
	assert.Empty(t, out, "при ошибке напечатано подтверждение")
	_, err = js.ObjectStore(t.Context(), record.Bucket(q.Seed, annaID))
	assert.ErrorIs(t, err, jetstream.ErrBucketNotFound, "для файла больше предела открыта корзина")
	assert.Zero(t, queued(t, stream), "запись о файле больше предела ушла в очередь")
}
