package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const keysPath = "/auth/v1/.well-known/jwks.json"

func (a *auth) routes(s Settings) http.Handler {
	ops := chi.NewRouter()
	ops.Post("/auth/v1/signup", a.signup)
	ops.Post("/auth/v1/token", a.token)
	ops.Post("/auth/v1/logout", a.logout)
	ops.Get("/rest/v1/records", a.records)
	ops.NotFound(operations.ServeHTTP)
	ops.MethodNotAllowed(operations.ServeHTTP)
	r := chi.NewRouter()
	keys := func(w http.ResponseWriter, _ *http.Request) { respond(w, http.StatusOK, string(a.keys)) }
	r.Get(keysPath, keys)
	r.Head(keysPath, keys)
	r.NotFound(guard(s, ops).ServeHTTP)
	r.MethodNotAllowed(guard(s, ops).ServeHTTP)
	return http.MaxBytesHandler(r, 1<<20)
}
