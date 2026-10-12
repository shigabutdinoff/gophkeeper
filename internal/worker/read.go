package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

var maxSealed uint64 = record.MaxSealed

func read(ctx context.Context, js jetstream.JetStream, obs jetstream.ObjectStore, bucket string, meta *jetstream.ObjectInfo, c record.Change) ([]byte, error) {
	parts := "$O." + bucket + ".C." + meta.NUID
	forged := fmt.Errorf("%w: файл в очереди не тот, что в записи, или не читается", record.ErrForged)
	linked := meta.Opts != nil && meta.Opts.Link != nil
	if meta.Digest != c.Digest || meta.Size > maxSealed || meta.NUID == "" || strings.ContainsAny(meta.NUID, " .*>") || linked {
		return nil, forged
	}
	stream, err := js.Stream(ctx, "OBJ_"+bucket)
	if err != nil {
		return nil, err
	}
	info, err := stream.Info(ctx, jetstream.WithSubjectFilter(parts))
	if err != nil {
		return nil, err
	}
	if info.State.Subjects[parts] != uint64(meta.Chunks) || (meta.Chunks == 0) != (meta.Size == 0) {
		return nil, forged
	}
	obj, err := obs.Get(ctx, c.Record)
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, int64(maxSealed)+1))
	if errors.Is(err, jetstream.ErrDigestMismatch) || err == nil && (uint64(len(data)) > maxSealed || record.Digest(data) != c.Digest) {
		return nil, forged
	}
	return data, err
}
