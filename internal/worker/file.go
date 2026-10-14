package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func transfer(ctx context.Context, js jetstream.JetStream, msg jetstream.Msg, s Server, c record.Change) (jetstream.ObjectStore, error) {
	if c.Digest == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, fileAttempt)
	defer cancel()
	go extend(ctx, msg, s.tuned().ackWait/3)
	name := record.Bucket(s.Seed, c.Owner)
	obs, err := js.ObjectStore(ctx, name)
	var meta *jetstream.ObjectInfo
	if err == nil {
		meta, err = obs.GetInfo(ctx, c.Record)
	}
	if errors.Is(err, jetstream.ErrBucketNotFound) || errors.Is(err, jetstream.ErrObjectNotFound) || errors.Is(err, jetstream.ErrBadObjectMeta) {
		return nil, fmt.Errorf("%w, файла нет в очереди", errSkipped)
	}
	if err != nil {
		return nil, err
	}
	data, err := read(ctx, js, obs, name, meta, c, s.tuned().maxSealed)
	if errors.Is(err, record.ErrForged) {
		if derr := obs.Delete(ctx, c.Record); derr != nil {
			err = fmt.Errorf("%w, объект из корзины не удалён: %v", err, derr)
		}
		return nil, err
	}
	if err == nil {
		err = s.Files.Put(ctx, c.Owner, c.Record, data)
	}
	return obs, err
}
