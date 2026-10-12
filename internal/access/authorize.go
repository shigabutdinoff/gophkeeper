package access

import (
	"errors"
	"fmt"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	natsjwt "github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	callout "github.com/synadia-io/callout.go"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func authorizer(issuer nkeys.KeyPair, seed, account, iss string, keys jwt.Keyfunc) callout.AuthorizerFn {
	return func(req *natsjwt.AuthorizationRequest) (string, error) {
		var claims jwt.RegisteredClaims
		_, err := jwt.ParseWithClaims(req.ConnectOptions.Token, &claims, keys, jwt.WithValidMethods([]string{"ES256"}),
			jwt.WithIssuer(iss), jwt.WithAudience("authenticated"), jwt.WithExpirationRequired())
		if err != nil {
			return "", fmt.Errorf("токен доступа не принят: %w", err)
		}
		owner, err := uuid.Parse(claims.Subject)
		if err != nil {
			return "", errors.New("токен доступа не принят: пользователь не UUID")
		}
		uc := natsjwt.NewUserClaims(req.UserNkey)
		uc.Audience = natsserver.DEFAULT_GLOBAL_ACCOUNT
		if account != "" {
			uc.Audience, uc.IssuerAccount = "", account
		}
		o, b := owner.String(), record.Bucket(seed, owner.String())
		uc.Pub.Allow.Add(record.Topic(o), record.FilesTopic(o), "$O."+b+".>", "$JS.API.STREAM.INFO.OBJ_"+b,
			"$JS.API.STREAM.PURGE.OBJ_"+b, "$JS.API.DIRECT.GET.OBJ_"+b+".>")
		uc.Sub.Allow.Add(nats.InboxPrefix + o + ".>")
		uc.Expires = claims.ExpiresAt.Unix()
		return uc.Encode(issuer)
	}
}
