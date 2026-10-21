package server

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-jwt/jwt/v5/request"
)

func (a *auth) bearer(r *http.Request, from request.Extractor) (jwt.RegisteredClaims, error) {
	var claims jwt.RegisteredClaims
	_, err := request.ParseFromRequest(r, from,
		func(*jwt.Token) (any, error) { return &a.key.PublicKey, nil }, request.WithClaims(&claims), request.WithParser(jwt.NewParser(jwt.WithValidMethods([]string{"ES256"}), jwt.WithExpirationRequired())))
	return claims, err
}
