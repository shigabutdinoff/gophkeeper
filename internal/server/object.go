package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func (a *auth) object(w http.ResponseWriter, r *http.Request, name, resource string) {
	switch r.Method {
	case http.MethodPut:
		a.store(w, r, name, resource)
	case http.MethodGet:
		obj, err := a.files.Get(r.Context(), name)
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			noObject().send(w, resource)
		} else if a.failed(w, r, err, resource) {
			defer func() { _ = obj.Close() }()
			info, _ := obj.Info()
			h := w.Header()
			h.Set("Cache-Control", "no-cache")
			h.Set("Content-Length", strconv.FormatUint(info.Size, 10))
			h.Set("Content-Type", "application/octet-stream")
			h.Set("ETag", strconv.Quote(info.Digest))
			h.Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
			_, _ = io.Copy(w, obj)
		}
	default:
		if err := a.files.Delete(r.Context(), name); errors.Is(err, jetstream.ErrObjectNotFound) || a.failed(w, r, err, resource) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func (a *auth) failed(w http.ResponseWriter, r *http.Request, err error, resource string) bool {
	if err != nil {
		a.log.Error("внутренняя ошибка хранилища файлов", zap.Error(err), zap.String("path", r.URL.Path))
		s3Failed().send(w, resource)
	}
	return err == nil
}
