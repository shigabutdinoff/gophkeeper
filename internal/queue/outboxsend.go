package queue

import (
	"context"
	"fmt"
)

// Drain отправляет изменения устройства в очередь изменений по порядку и
// удаляет подтверждённые не дольше 10 секунд. Отказ в доступе даёт
// ErrDenied, иначе ErrUnavailable.
func (o *Outbox) Drain(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	info, err := o.box.Info(ctx)
	switch {
	case err != nil:
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	case info.State.Msgs == 0:
		return nil
	case o.access == "":
		return ErrDenied
	}
	if err = o.wait(ctx); err != nil {
		return err
	}
	for seq := info.State.FirstSeq; seq <= info.State.LastSeq && err == nil; seq++ {
		err = o.send(ctx, seq)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return nil
}
