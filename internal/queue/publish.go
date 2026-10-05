package queue

import (
	"context"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var attempt = 2 * time.Second

func publish(ctx context.Context, nc *nats.Conn, msg *nats.Msg) error {
	up := nc.StatusChanged(nats.CONNECTED)
	if !nc.IsConnected() {
		select {
		case <-up:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	return retry.New(retry.Context(ctx), retry.UntilSucceeded(), retry.DelayType(retry.FixedDelay)).Do(func() error {
		try, cancel := context.WithTimeout(ctx, attempt)
		defer cancel()
		_, err := js.PublishMsg(try, msg)
		return err
	})
}
