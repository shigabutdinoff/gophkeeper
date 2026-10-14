package server

import (
	"crypto/subtle"
	"net/http"
)

const (
	noKey  = "{\"message\":\"No API key found in request\",\"hint\":\"No `apikey` request header or url param was found.\"}"
	badKey = "{\"message\":\"Invalid API key\",\"hint\":\"Double check your Supabase `anon` or `service_role` API key.\"}"
)

func guard(s Settings, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, body := []byte(r.Header.Get("apikey")), badKey
		switch {
		case len(key) == 0:
			body = noKey
		case subtle.ConstantTimeCompare(key, []byte(s.AppKey))|subtle.ConstantTimeCompare(key, []byte(s.ServerKey)) == 1:
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(body))
	})
}
