package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

// ErrNoQueue сообщает, что не заданы подключение к очереди или предел
// корзины файлов.
var ErrNoQueue = errors.New("нужны подключение к очереди и предел корзины файлов")

const keep = ".keep"

func bucket(ctx context.Context, s Settings) (jetstream.ObjectStore, error) {
	if s.Queue == nil || s.FilesLimit <= 0 {
		return nil, ErrNoQueue
	}
	store, err := s.Queue.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: record.Bucket(s.ServerKey, "files"), MaxBytes: s.FilesLimit})
	if err == nil {
		if _, err = store.GetInfo(ctx, keep); errors.Is(err, jetstream.ErrObjectNotFound) {
			_, err = store.PutBytes(ctx, keep, nil)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("корзина файлов не открыта: %w", err)
	}
	return store, nil
}
