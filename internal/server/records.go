package server

import (
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5/request"
)

type entry struct {
	ID     string `json:"id"`
	Note   string `json:"note"`
	Secret string `json:"secret"`
}

func (a *auth) records(w http.ResponseWriter, r *http.Request) {
	claims, err := a.bearer(r, request.AuthorizationHeaderExtractor)
	if err != nil {
		respond(w, http.StatusUnauthorized, jwtInvalid)
		return
	}
	where, limit, ok := filter(r.URL.Query())
	if !ok {
		respond(w, http.StatusBadRequest, syntax)
		return
	}
	list := []entry{}
	err = again(r.Context(), func() error {
		return a.db.WithContext(r.Context()).Table("records").Select("id", "note", "secret").Where("user_id = ?", claims.Subject).
			Where(where).Order("id").Limit(limit).Find(&list).Error
	})
	if err != nil {
		a.log.WithError(err).WithField("path", r.URL.Path).Error("внутренняя ошибка сервера")
		respond(w, http.StatusInternalServerError, internal)
		return
	}
	body, _ := json.Marshal(list)
	respond(w, http.StatusOK, string(body))
}
