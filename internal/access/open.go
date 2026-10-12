package access

import (
	"context"
	"errors"
	"fmt"
	"strings"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const bucketBytes = 100 << 20

func buckets(ctx context.Context, nc *nats.Conn, seed string, log natsserver.Logger, stop func() error) (func() error, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, errors.Join(err, stop())
	}
	sub, err := nc.Subscribe(record.FilesTopic("*"), func(msg *nats.Msg) {
		owner := strings.TrimPrefix(msg.Subject, record.FilesTopic(""))
		if !strings.HasPrefix(msg.Reply, nats.InboxPrefix+owner+".") {
			return
		}
		name := record.Bucket(seed, owner)
		sweep(ctx, js)
		if _, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: name, MaxBytes: bucketBytes}); err != nil {
			log.Errorf("корзина файлов %s не открыта: %v", owner, err)
		}
		msg.Respond([]byte(name))
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("служба корзин файлов: %w", err), stop())
	}
	return func() error { return errors.Join(sub.Unsubscribe(), stop()) }, nil
}
