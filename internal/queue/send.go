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

// Send отправляет изменение msg в очередь из настроек s с параметрами extra
// и ждёт подтверждения не дольше 10 секунд. Повтор того же msg не создаёт
// второго изменения. Отказ в доступе даёт ErrDenied, иначе ErrUnavailable.
func Send(ctx context.Context, s config.Settings, msg *nats.Msg, extra ...nats.Option) error {
	if err := Check(s); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	nc, err := Connect(s, extra...)
	if err == nil {
		defer nc.Close()
		err = publish(ctx, nc, msg)
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
