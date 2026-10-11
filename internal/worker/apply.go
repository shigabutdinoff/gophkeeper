package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

var (
	attempt    = 5 * time.Second
	errSkipped = errors.New("изменение пропущено")
)

func apply(ctx context.Context, s Server, c record.Change) error {
	ctx, cancel := context.WithTimeout(ctx, attempt)
	defer cancel()
	owned := map[string]string{"user_id": "eq." + c.Owner, "id": "eq." + c.Record}
	fields := map[string]any{"id": c.Record, "user_id": c.Owner, "note": c.Note, "secret": c.Secret, "updated_at": "now"}
	var done []row
	var err error
	switch c.Op {
	case "put":
		done, err = query(ctx, s, http.MethodPost, "records", map[string]string{"on_conflict": "user_id,id"}, fields)
	case "edit":
		done, err = query(ctx, s, http.MethodPatch, "records", owned, fields)
	case "delete":
		done, err = query(ctx, s, http.MethodDelete, "records", owned, nil)
	}
	if err == nil && len(done) == 0 {
		return fmt.Errorf("%w, записи нет у владельца", errSkipped)
	}
	return err
}
