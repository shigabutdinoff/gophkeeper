package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func (a *auth) store(w http.ResponseWriter, r *http.Request, name, resource string) {
	if r.ContentLength > objectLimit {
		tooLarge().send(w, resource)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()
	info, err := a.files.Put(ctx, jetstream.ObjectMeta{Name: name}, r.Body)
	full := errors.Is(err, &jetstream.APIError{ErrorCode: 10077})
	if full {
		_ = a.files.Delete(ctx, name)
	}
	if _, over := errors.AsType[*http.MaxBytesError](err); over || full {
		tooLarge().send(w, resource)
	} else if a.failed(w, r, err, resource) {
		w.Header().Set("ETag", strconv.Quote(info.Digest))
	}
}
