package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const fileAttempt = 5 * time.Minute

var errSkipped = errors.New("изменение пропущено")

func apply(ctx context.Context, js jetstream.JetStream, msg jetstream.Msg, s Server, c record.Change) error {
	obs, err := transfer(ctx, js, msg, s, c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.tuned().attempt)
	defer cancel()
	owned := map[string]string{"user_id": "eq." + c.Owner, "id": "eq." + c.Record}
	fields := map[string]any{"id": c.Record, "user_id": c.Owner, "note": c.Note, "secret": c.Secret, "updated_at": "now"}
	var done []row
	switch c.Op {
	case "put":
		done, err = query(ctx, s, http.MethodPost, "records", map[string]string{"on_conflict": "user_id,id"}, fields)
	case "edit":
		done, err = query(ctx, s, http.MethodPatch, "records", owned, fields)
	case "delete":
		if err = s.Files.Delete(ctx, c.Owner, c.Record); err == nil {
			done, err = query(ctx, s, http.MethodDelete, "records", owned, nil)
		}
	}
	if err == nil && len(done) == 0 {
		err = fmt.Errorf("%w, записи нет у владельца", errSkipped)
	}
	return discard(ctx, s, obs, c, err)
}
