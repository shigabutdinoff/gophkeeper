package queue

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const fileDeadline = 5 * time.Minute

// Upload просит очередь из настроек s открыть корзину владельца owner и
// кладёт в неё зашифрованный файл data записи id. Срок отправки 5 минут,
// повтора нет, ошибки те же, что у Send.
func Upload(ctx context.Context, s config.Settings, owner, id string, data []byte, extra ...nats.Option) error {
	return session(ctx, s, fileDeadline, extra, func(ctx context.Context, nc *nats.Conn) error {
		open, cancel := context.WithTimeout(ctx, deadline)
		defer cancel()
		js, err := jetstream.New(nc)
		opened, rerr := nc.RequestWithContext(open, record.FilesTopic(owner), nil)
		if err != nil || rerr != nil {
			return errors.Join(err, rerr)
		}
		obs, err := js.ObjectStore(ctx, string(opened.Data))
		if err == nil {
			_, err = obs.PutBytes(ctx, id, data)
		}
		return err
	})
}
