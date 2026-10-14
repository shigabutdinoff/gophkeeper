package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const keysPath, filesPath = "/auth/v1/.well-known/jwks.json", "/storage/v1/s3/files/*"

func (a *auth) routes(s Settings) http.Handler {
	ops := chi.NewRouter()
	ops.Post("/auth/v1/signup", a.signup)
	ops.Post("/auth/v1/token", a.token)
	ops.Post("/auth/v1/logout", a.logout)
	ops.Get("/rest/v1/records", a.records)
	ops.Post("/rest/v1/records", a.write(s))
	ops.Patch("/rest/v1/records", a.write(s))
	ops.Delete("/rest/v1/records", a.write(s))
	ops.NotFound(operations.ServeHTTP)
	ops.MethodNotAllowed(operations.ServeHTTP)
	r := chi.NewRouter()
	keys := func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, string(a.keys)) }
	r.Get(keysPath, keys)
	r.Head(keysPath, keys)
	file := http.MaxBytesHandler(a.file(s), objectLimit)
	r.Method(http.MethodPut, filesPath, file)
	r.Method(http.MethodGet, filesPath, file)
	r.Method(http.MethodDelete, filesPath, file)
	limited := http.MaxBytesHandler(guard(s, ops), 1<<20)
	r.NotFound(limited.ServeHTTP)
	r.MethodNotAllowed(limited.ServeHTTP)
	return r
}
