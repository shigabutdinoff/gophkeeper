package server

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

func (a *auth) signup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !a.decode(r, &in) {
		respond(w, http.StatusBadRequest, invalid)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		respond(w, http.StatusBadRequest, invalid)
		return
	}
	acc := account{Email: strings.ToLower(in.Email), EncryptedPassword: string(hash)}
	res := a.db.WithContext(r.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&acc)
	switch {
	case res.Error != nil:
		respond(w, http.StatusInternalServerError, failed)
	case res.RowsAffected == 0:
		respond(w, http.StatusUnprocessableEntity, exists)
	default:
		a.admit(w, r, acc, false)
	}
}
