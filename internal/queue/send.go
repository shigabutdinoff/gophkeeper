package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const deadline = 10 * time.Second

func session(ctx context.Context, s config.Settings, limit time.Duration, extra []nats.Option, do func(context.Context, *nats.Conn) error) error {
	if err := Check(s); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	nc, err := Connect(s, extra...)
	if err == nil {
		defer nc.Close()
		err = do(ctx, nc)
		if last := nc.LastError(); err != nil && (errors.Is(last, nats.ErrAuthorization) || errors.Is(last, nats.ErrAuthExpired)) {
			return fmt.Errorf("%w: %w", ErrDenied, last)
		}
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return nil
}

// Connect подключается к очереди из настроек s с параметрами extra.
// Возвращает ErrNoQueue без адреса и ErrInsecure без защищённого соединения.
func Connect(s config.Settings, extra ...nats.Option) (*nats.Conn, error) {
	opts, err := options(s)
	if err != nil {
		return nil, err
	}
	return nats.Connect(s.Queue, append(opts, extra...)...)
}
