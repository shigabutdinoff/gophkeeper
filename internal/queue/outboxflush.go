package queue

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

// Flush отправляет изменения, сохранённые на устройстве пользователем user,
// и возвращает последнее оставшееся изменение каждой записи вместе с ошибкой
// отправки. Параметры такие же, как у Open.
func Flush(ctx context.Context, s config.Settings, user, data, access string) (map[string]record.Change, error) {
	root, err := outboxRoot()
	if err != nil || s.QueueLeaf == "" {
		return nil, err
	}
	if _, err = os.Stat(filepath.Join(root, user)); err != nil {
		return nil, nil
	}
	o, err := Open(ctx, s, user, data, access)
	if err != nil {
		return nil, err
	}
	defer o.Close()
	err = o.Drain(ctx)
	return o.left(ctx), err
}

// Kept сообщает, остались ли на устройстве неотправленные изменения.
func Kept() bool {
	root, err := outboxRoot()
	entries, _ := os.ReadDir(root)
	return err == nil && slices.ContainsFunc(entries, fs.DirEntry.IsDir)
}
