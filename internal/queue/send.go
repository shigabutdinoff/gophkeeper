package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const deadline = 10 * time.Second

// Send отправляет изменение msg в очередь из настроек s и ждёт подтверждения
// не дольше 10 секунд. Повтор того же msg не создаёт второго изменения.
// Если подтверждения нет, возвращает ErrUnavailable.
func Send(ctx context.Context, s config.Settings, msg *nats.Msg) error {
	opts, err := options(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	nc, err := nats.Connect(s.Queue, opts...)
	if err == nil {
		defer nc.Close()
		err = publish(ctx, nc, msg)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return nil
}
