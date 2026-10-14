package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func attempt() time.Duration {
	if testing.Testing() {
		return 200 * time.Millisecond
	}
	return 2 * time.Second
}

func publish(ctx context.Context, nc *nats.Conn, msg *nats.Msg) error {
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	return retry.New(retry.Context(ctx), retry.UntilSucceeded(), retry.DelayType(retry.FixedDelay),
		retry.RetryIf(func(err error) bool { return !errors.Is(err, nats.ErrConnectionClosed) }),
	).Do(func() error {
		try, cancel := context.WithTimeout(ctx, attempt())
		defer cancel()
		_, err := js.PublishMsg(try, msg)
		return err
	})
}

// Send отправляет изменение msg в очередь из настроек s с параметрами extra
// и ждёт подтверждения не дольше 10 секунд. Повтор того же msg не создаёт
// второго изменения. Отказ в доступе даёт ErrDenied, иначе ErrUnavailable.
func Send(ctx context.Context, s config.Settings, msg *nats.Msg, extra ...nats.Option) error {
	return session(ctx, s, deadline, extra, func(ctx context.Context, nc *nats.Conn) error { return publish(ctx, nc, msg) })
}
