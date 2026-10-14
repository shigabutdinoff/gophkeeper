package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func logoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Выйти на этом устройстве",
		Long:  "Завершает вход на этом устройстве и удаляет ключ к данным из системного хранилища секретов.\nДругие устройства остаются в системе.",
		Args:  NoArgs,
	}
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		var s config.Settings
		if dir, err := config.Dir(); err == nil {
			s, _ = config.Load[config.Settings](filepath.Join(dir, config.File))
		}
		if err := account.Logout(c.Context(), httpClient, s); err != nil {
			return err
		}
		return report(c, "Выход выполнен.")
	}
	return cmd
}
