package server

import (
	"crypto/subtle"
	"net/http"
)

func files(s Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Amz-Security-Token")), []byte(s.ServerKey)) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}
}
