package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go/v5"
)

var leafWait = 3 * time.Second

var errNoLeaf = errors.New("очередь устройства не связана с очередью изменений")

type denial struct{ atomic.Bool }

func (d *denial) Noticef(string, ...any) {}
func (d *denial) Warnf(string, ...any)   {}
func (d *denial) Fatalf(string, ...any)  {}
func (d *denial) Debugf(string, ...any)  {}
func (d *denial) Tracef(string, ...any)  {}

func (d *denial) Errorf(format string, v ...any) {
	if strings.Contains(fmt.Sprintf(format, v...), "Authorization Violation") {
		d.Store(true)
	}
}

func (o *Outbox) wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, leafWait)
	defer cancel()
	err := retry.New(retry.Context(ctx), retry.UntilSucceeded(), retry.Delay(50*time.Millisecond), retry.DelayType(retry.FixedDelay),
		retry.RetryIf(func(err error) bool { return !errors.Is(err, ErrDenied) }), retry.LastErrorOnly(true)).Do(func() error {
		switch {
		case o.denied.Load():
			return ErrDenied
		case o.srv.NumLeafNodes() == 0:
			return errNoLeaf
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrDenied) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}
