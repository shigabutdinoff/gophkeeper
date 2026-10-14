package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/shigabutdinoff/gophkeeper/api"
)

const keysPath, filesPath = "/auth/v1/.well-known/jwks.json", "/storage/v1/s3/files/*"

func (a *auth) routes(s Settings) http.Handler {
	ops := chi.NewRouter()
	a.authRoutes(ops)
	a.recordRoutes(ops)
	a.writeRoutes(ops, s)
	ops.NotFound(operations.ServeHTTP)
	ops.MethodNotAllowed(operations.ServeHTTP)
	r := chi.NewRouter()
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, string(api.Swagger)) })
	keys := func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, string(a.keys)) }
	r.Get(keysPath, keys)
	r.Head(keysPath, keys)
	a.fileRoutes(r, s)
	limited := http.MaxBytesHandler(guard(s, ops), 1<<20)
	r.NotFound(limited.ServeHTTP)
	r.MethodNotAllowed(limited.ServeHTTP)
	return r
}
