package access

import (
	"context"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

const refresh, wait = 10 * time.Minute, time.Second

func serverKeys(ctx context.Context, base string, client *http.Client, log natsserver.Logger) (keyfunc.Keyfunc, error) {
	return keyfunc.NewDefaultOverrideCtx(ctx, []string{base + "/.well-known/jwks.json"}, keyfunc.Override{
		Client: client, HTTPTimeout: 10 * time.Second, RefreshInterval: refresh, RateLimitWaitMax: wait,
		RefreshErrorHandlerFunc: func(string) func(context.Context, error) {
			return func(_ context.Context, err error) {
				log.Errorf("ключи сервера не обновлены: %v", err)
			}
		},
	})
}
