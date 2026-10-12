package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
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
	return online{s, keys}, err
}
