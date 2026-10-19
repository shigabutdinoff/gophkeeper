package server

import (
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const decoy = "$2a$10$ly8eVL9M7SbG0WuETy8p.eP.oAWsZQUJnQ2bgUyvDEmkBsqpM24Me"

func (a *auth) token(w http.ResponseWriter, r *http.Request) {
	grant := r.URL.Query().Get("grant_type")
	if grant == "refresh_token" {
		a.refresh(w, r)
		return
	}
	var in credentials
	if !a.decode(r, &in) || grant != "password" {
		respond(w, http.StatusBadRequest, invalid)
		return
	}
	var acc account
	err := a.db.WithContext(r.Context()).Take(&acc, "email = ?", strings.ToLower(in.Email)).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		_ = bcrypt.CompareHashAndPassword([]byte(decoy), []byte(in.Password))
		respond(w, http.StatusBadRequest, wrong)
	case err != nil:
		respond(w, http.StatusInternalServerError, failed)
	case bcrypt.CompareHashAndPassword([]byte(acc.EncryptedPassword), []byte(in.Password)) != nil:
		respond(w, http.StatusBadRequest, wrong)
	default:
		a.admit(w, r, acc, true)
	}
}
