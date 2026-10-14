package access

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func idle() time.Duration {
	if testing.Testing() {
		return 300 * time.Millisecond
	}
	return time.Minute
}

func sweep(ctx context.Context, js jetstream.JetStream) {
	for name := range js.ObjectStoreNames(ctx).Name() {
		obs, err := js.ObjectStore(ctx, name)
		if err != nil {
			continue
		}
		status, _ := obs.Status(ctx)
		st, ok := status.(*jetstream.ObjectBucketStatus)
		if !ok || time.Since(st.StreamInfo().Created) <= idle() || time.Since(st.StreamInfo().State.LastTime) <= idle() {
			continue
		}
		if _, err = obs.List(ctx); errors.Is(err, jetstream.ErrNoObjectsFound) {
			_ = js.DeleteObjectStore(ctx, name)
		}
	}
}
