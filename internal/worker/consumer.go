package worker

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const durable = "gophkeeper-worker"

var (
	ackWait   = 30 * time.Second
	heartbeat = 5 * time.Second
)

func consume(ctx, work context.Context, js jetstream.JetStream, s Server) error {
	filter := record.Topic("*")
	stream, err := js.StreamNameBySubject(ctx, filter)
	if err != nil {
		return err
	}
	c, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{Durable: durable, FilterSubject: filter,
		AckPolicy: jetstream.AckExplicitPolicy, MaxAckPending: 1, AckWait: ackWait})
	if err != nil {
		return err
	}
	for {
		msg, err := c.Next(jetstream.FetchContext(ctx), jetstream.FetchHeartbeat(heartbeat))
		switch {
		case err == nil:
			handle(work, js, msg, s)
		case ctx.Err() != nil:
			return nil
		case !errors.Is(err, nats.ErrTimeout):
			return err
		}
	}
}
