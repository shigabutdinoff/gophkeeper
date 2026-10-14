package worker

import (
	"context"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func graceful(ctx context.Context, grace time.Duration) (context.Context, context.CancelFunc) {
	work, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, func() { time.AfterFunc(grace, cancel) })
	return work, func() { stop(); cancel() }
}

func extend(ctx context.Context, msg jetstream.Msg, every time.Duration) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-time.After(every):
			_ = msg.InProgress()
		}
	}
}
