package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

var maxDelay = 5 * time.Second

func handle(ctx context.Context, js jetstream.JetStream, msg jetstream.Msg, s Server) {
	var seq uint64
	if meta, err := msg.Metadata(); err == nil {
		seq = meta.Sequence.Stream
	}
	c, err := record.Parse(msg.Subject(), msg.Data())
	if h := msg.Headers(); h.Get(jetstream.StreamHeader) != "" || h.Get(jetstream.MsgIDHeader) == "" {
		err = fmt.Errorf("%w: ответ чтения из потока", record.ErrForged)
	}
	if err == nil {
		err = retry.New(retry.Context(ctx), retry.UntilSucceeded(), retry.MaxDelay(maxDelay), retry.LastErrorOnly(true),
			retry.RetryIf(func(err error) bool { return !errors.Is(err, errSkipped) && !errors.Is(err, record.ErrForged) }),
			retry.OnRetry(func(_ uint, err error) {
				s.Log.Printf("изменение %d не применено, повтор: %v", seq, err)
			}),
		).Do(func() error {
			msg.InProgress()
			return apply(ctx, js, msg, s, c)
		})
	}
	switch {
	case errors.Is(err, record.ErrForged):
		s.Log.Printf("изменение %d, тема %.64q: %v", seq, msg.Subject(), err)
		msg.TermWithReason(err.Error())
	case errors.Is(err, errSkipped):
		s.Log.Printf("изменение %d, запись %s: %v", seq, c.Record, err)
		msg.DoubleAck(ctx)
	case err != nil:
		msg.Nak()
	default:
		msg.DoubleAck(ctx)
	}
}
