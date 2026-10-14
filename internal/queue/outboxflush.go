package queue

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Flush отправляет изменения, сохранённые на устройстве пользователем user,
// если они есть. Параметры такие же, как у Open.
func Flush(ctx context.Context, s config.Settings, user, data, access string) error {
	root, err := outboxRoot()
	if err != nil || s.QueueLeaf == "" {
		return err
	}
	if _, err = os.Stat(filepath.Join(root, user)); err != nil {
		return nil
	}
	o, err := Open(ctx, s, user, data, access)
	if err != nil {
		return err
	}
	defer o.Close()
	return o.Drain(ctx)
}

// Kept сообщает, остались ли на устройстве неотправленные изменения.
func Kept() bool {
	root, err := outboxRoot()
	entries, _ := os.ReadDir(root)
	return err == nil && slices.ContainsFunc(entries, fs.DirEntry.IsDir)
}
