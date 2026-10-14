package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
)

const lifetime = time.Hour

func loadKey(path string) (*auth, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	key, err := jwt.ParseECPrivateKeyFromPEM(pem)
	if err != nil {
		return nil, err
	}
	public, err := jwk.Import(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	x, _ := public.(jwk.ECDSAPublicKey).X()
	kid := base64.RawURLEncoding.EncodeToString(x)
	err = errors.Join(public.Set(jwk.AlgorithmKey, jwa.ES256()), public.Set(jwk.KeyUsageKey, jwk.ForSignature), public.Set(jwk.KeyIDKey, kid))
	keys, _ := json.Marshal(map[string][]jwk.Key{"keys": {public}})
	log := logrus.New()
	log.SetOutput(io.Discard)
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	return &auth{key: key, kid: kid, keys: keys, log: log, validate: validator.New()}, err
}

func (a *auth) sign(issuer, subject, id string, expires time.Time) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{Issuer: issuer, Subject: subject, ID: id,
		Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(expires)})
	t.Header["kid"] = a.kid
	return t.SignedString(a.key)
}
