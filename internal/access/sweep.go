package access

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var idle = time.Minute

func sweep(ctx context.Context, js jetstream.JetStream) {
	for name := range js.ObjectStoreNames(ctx).Name() {
		obs, err := js.ObjectStore(ctx, name)
		if err != nil {
			continue
		}
		status, _ := obs.Status(ctx)
		st, ok := status.(*jetstream.ObjectBucketStatus)
		if !ok || time.Since(st.StreamInfo().Created) <= idle || time.Since(st.StreamInfo().State.LastTime) <= idle {
			continue
		}
		if _, err = obs.List(ctx); errors.Is(err, jetstream.ErrNoObjectsFound) {
			js.DeleteObjectStore(ctx, name)
		}
	}
}
