package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type credentials struct {
	Email    string `json:"email" validate:"email"`
	Password string `json:"password" validate:"required"`
}

func decode(r *http.Request, in any) bool {
	return json.NewDecoder(r.Body).Decode(in) == nil && validate.Struct(in) == nil
}
