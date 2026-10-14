package server

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func (a *auth) token(w http.ResponseWriter, r *http.Request) {
	grant := r.URL.Query().Get("grant_type")
	if grant == "refresh_token" {
		a.refresh(w, r)
		return
	}
	var in credentials
	if !decode(r, &in) || grant != "password" {
		respond(w, http.StatusBadRequest, invalid)
		return
	}
	var acc account
	err := a.db.WithContext(r.Context()).Take(&acc, "email = ?", strings.ToLower(in.Email)).Error
	switch {
	case err != nil:
		respond(w, http.StatusInternalServerError, failed)
	case bcrypt.CompareHashAndPassword([]byte(acc.EncryptedPassword), []byte(in.Password)) != nil:
		respond(w, http.StatusBadRequest, wrong)
	default:
		a.admit(w, r, acc, true)
	}
}
