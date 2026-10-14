package worker

import (
	"cmp"
	"context"
	"errors"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func discard(ctx context.Context, s Server, obs jetstream.ObjectStore, c record.Change, err error) error {
	if obs == nil || err != nil && !errors.Is(err, errSkipped) {
		return err
	}
	if errors.Is(err, errSkipped) {
		err = cmp.Or(s.Files.Delete(ctx, c.Owner, c.Record), err)
	}
	return cmp.Or(obs.Delete(ctx, c.Record), err)
}
