package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

func awaitQueue(ctx context.Context, q queueAccess, exited <-chan error, logPath string) error {
	deadline := time.After(startTimeout)
	for {
		nc, err := nats.Connect(q.URL, q.Admin, nats.NoReconnect())
		if err == nil {
			nc.Close()
			return prepareQueue(ctx, q)
		}
		if !notRunning(err) {
			return fmt.Errorf("подключение к очереди: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("ожидание очереди песочницы прервано: %w", ctx.Err())
		case werr := <-exited:
			return fmt.Errorf("очередь песочницы остановилась (%v), подробности в %q: %w", werr, logPath, err)
		case <-deadline:
			return fmt.Errorf("очередь песочницы не готова за %s, подробности в %q: %w", startTimeout, logPath, err)
		case <-time.After(startPoll):
		}
	}
}
