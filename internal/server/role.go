package server

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

const denied = `{"code":"42501","details":null,"hint":"Grant the required privileges to the current role with: GRANT %s ON public.records TO %s;","message":"permission denied for table records"}`

var grants = map[string]string{http.MethodPost: "INSERT, UPDATE", http.MethodPatch: "UPDATE", http.MethodDelete: "DELETE"}

func (a *auth) writer(w http.ResponseWriter, r *http.Request, s Settings) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case subtle.ConstantTimeCompare([]byte(token), []byte(s.ServerKey)) == 1:
		return true
	case token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.AppKey)) == 1:
		w.Header().Set("WWW-Authenticate", "Bearer")
		refuse(w, http.StatusUnauthorized, "SELECT, "+grants[r.Method], "anon")
	default:
		if _, err := a.bearer(r); err != nil {
			respond(w, http.StatusUnauthorized, jwtInvalid)
			return false
		}
		refuse(w, http.StatusForbidden, grants[r.Method], "authenticated")
	}
	return false
}

func refuse(w http.ResponseWriter, status int, grant, role string) {
	w.Header().Set("Proxy-Status", "PostgREST; error=42501")
	respond(w, status, fmt.Sprintf(denied, grant, role))
}
