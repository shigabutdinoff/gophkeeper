package server

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
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
	jwk, err := jwkset.NewJWKFromKey(key, jwkset.JWKOptions{Metadata: jwkset.JWKMetadataOptions{ALG: jwkset.AlgES256, USE: jwkset.UseSig}})
	if err != nil {
		return nil, err
	}
	public := jwk.Marshal()
	public.KID = public.X
	keys, err := json.Marshal(jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{public}})
	return &auth{key: key, kid: public.KID, keys: keys}, err
}

func (a *auth) sign(issuer, subject, id string, expires time.Time) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{Issuer: issuer, Subject: subject, ID: id,
		Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(expires)})
	t.Header["kid"] = a.kid
	return t.SignedString(a.key)
}
