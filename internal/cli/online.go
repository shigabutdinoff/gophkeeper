package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

type online struct {
	s    config.Settings
	keys account.Secrets
}

func connect(c *cobra.Command) (online, error) {
	s, err := resolve("")
	if err != nil {
		return online{}, err
	}
	keys, err := account.Unlock(c.Context(), httpClient, s, func(email string) (string, error) {
		return askHidden(c, "Пароль "+email)
	})
	if err == nil && keys.Access != "" {
		_ = queue.Flush(c.Context(), s, keys.User, keys.Data, keys.Access)
	}
	return online{s, keys}, err
}
