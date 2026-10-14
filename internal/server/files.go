package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5/request"
)

const objectLimit = 50 << 20

func (a *auth) file(s Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "*")
		owner, id, _ := strings.Cut(key, "/")
		resource, token := "files/"+key, r.Header.Get("X-Amz-Security-Token")
		var refused s3Error
		if token == "" {
			refused = noToken
		} else if subtle.ConstantTimeCompare([]byte(token), []byte(s.ServerKey)) != 1 {
			claims, err := a.bearer(r, request.HeaderExtractor{"X-Amz-Security-Token"})
			refused = map[string]s3Error{http.MethodPut: putDenied, http.MethodDelete: deleteDenied}[r.Method]
			if err != nil {
				refused = badToken
			} else if claims.Subject != owner {
				id = ""
			}
		}
		if refused.status == 0 && (id == "" || strings.Contains(id, "/")) {
			refused = noObject
		}
		if refused.status != 0 {
			refused.send(w, resource)
			return
		}
		a.object(w, r, key, resource)
	}
}
