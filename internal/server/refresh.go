package server

import (
	"errors"
	"net/http"

	"github.com/golang-jwt/jwt/v5/request"
	"gorm.io/gorm"
)

func (a *auth) refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"refresh_token" validate:"required"`
	}
	if !a.decode(r, &in) {
		respond(w, http.StatusBadRequest, invalid)
		return
	}
	var s session
	err := a.again(r.Context(), func() error {
		return a.db.WithContext(r.Context()).InnerJoins("Account").Take(&s, "refresh_token = ?", in.Token).Error
	})
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		respond(w, http.StatusBadRequest, gone)
	case err != nil:
		respond(w, http.StatusInternalServerError, failed)
	default:
		a.grant(w, r, s.Account, s, true, nil)
	}
}

func (a *auth) logout(w http.ResponseWriter, r *http.Request) {
	claims, err := a.bearer(r, request.AuthorizationHeaderExtractor)
	switch {
	case err != nil || r.URL.Query().Get("scope") != "local":
		respond(w, http.StatusBadRequest, invalid)
	case a.db.WithContext(r.Context()).Delete(&session{}, "id = ?", claims.ID).Error != nil:
		respond(w, http.StatusInternalServerError, failed)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
