package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	callout "github.com/synadia-io/callout.go"
)

// Serve запускает на соединении nc службу доступа к очереди и возвращает её
// остановку. Токены сервера server проверяются по его ключам, ответы
// подписывает seed. Непустой account включает режим оператора очереди.
func Serve(ctx context.Context, nc *nats.Conn, seed, account, server string,
	client *http.Client, log natsserver.Logger) (func() error, error) {
	issuer, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("ключ выдачи доступа к очереди: %w", err)
	}
	if account != "" && !nkeys.IsValidPublicAccountKey(account) {
		return nil, errors.New("учётная запись очереди задана не открытым ключом")
	}
	base := strings.TrimSuffix(server, "/") + "/auth/v1"
	keys, err := serverKeys(ctx, base, client, log)
	if err != nil {
		return nil, fmt.Errorf("ключи сервера: %w", err)
	}
	opts := []callout.Option{callout.Authorizer(authorizer(issuer, account, base, keys.Keyfunc)),
		callout.ResponseSignerKey(issuer), callout.Logger(log)}
	if account != "" {
		opts = append(opts, callout.ResponseSignerIssuer(account))
	}
	svc, err := callout.NewAuthorizationService(nc, opts...)
	if err != nil {
		return nil, fmt.Errorf("служба доступа к очереди: %w", err)
	}
	return svc.Stop, nil
}
