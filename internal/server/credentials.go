package server

import (
	"encoding/json"
	"net/http"
)

type credentials struct {
	Email    string `json:"email" validate:"email,max=254"`
	Password string `json:"password" validate:"required"`
}

func (a *auth) decode(r *http.Request, in any) bool {
	return json.NewDecoder(r.Body).Decode(in) == nil && a.validate.Struct(in) == nil
}
