package queue

import (
	"context"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func (o *Outbox) left(ctx context.Context) map[string]record.Change {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deadline)
	defer cancel()
	left := map[string]record.Change{}
	for seq := uint64(1); ; {
		raw, err := o.box.GetMsg(ctx, seq, jetstream.WithGetMsgSubject(">"))
		if err != nil {
			return left
		}
		if c, err := record.Parse(strings.TrimPrefix(raw.Subject, local), raw.Data); err == nil {
			left[c.Record] = c
		}
		seq = raw.Sequence + 1
	}
}
