package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

var budget = 9 * time.Second

type online struct {
	ctx  context.Context
	s    config.Settings
	keys account.Secrets
	stop func()
}

func connect(c *cobra.Command) (online, error) {
	s, err := resolve("")
	if err != nil {
		return online{stop: func() {}}, err
	}
	ctx, cancel := context.WithCancelCause(c.Context())
	timer := time.AfterFunc(budget, func() { cancel(context.DeadlineExceeded) })
	stop := func() { timer.Stop(); cancel(nil) }
	keys, err := account.Unlock(ctx, httpClient, s, func(email string) (string, error) {
		timer.Stop()
		defer timer.Reset(budget)
		return askHidden(c, "Пароль "+email)
	})
	return online{ctx, s, keys, stop}, err
}
