package queuetest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

// Sign подписывает claims ключом токенов очереди по ES256.
func (q *Queue) Sign(t testing.TB, claims jwt.Claims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(q.key)
	require.NoError(t, err, "токен доступа не подписан")
	return signed
}

// Token возвращает токен доступа сервера Supabase для пользователя owner
// со сроком ttl.
func (q *Queue) Token(t testing.TB, owner string, ttl time.Duration) string {
	t.Helper()
	return q.Sign(t, jwt.RegisteredClaims{Issuer: q.Supabase + "/auth/v1", Subject: owner,
		Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl))})
}

// As возвращает параметры подключения устройства, на котором час назад
// вошёл пользователь owner.
func (q *Queue) As(t testing.TB, owner string) nats.Option {
	t.Helper()
	token := q.Token(t, owner, time.Hour)
	return func(o *nats.Options) error {
		o.Token, o.InboxPrefix = token, nats.InboxPrefix+owner
		return nil
	}
}

// JWKS отдаёт открытый ключ токенов очереди, как сервер Supabase.
func (q *Queue) JWKS(w http.ResponseWriter, _ *http.Request) {
	point, _ := q.key.PublicKey.Bytes()
	x, y := base64.RawURLEncoding.EncodeToString(point[1:33]), base64.RawURLEncoding.EncodeToString(point[33:])
	json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "EC", "crv": "P-256", "alg": "ES256", "x": x, "y": y}}})
}
