package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func logoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Выйти на этом устройстве",
		Long:  "Завершает вход на этом устройстве и удаляет ключ к данным из системного хранилища секретов.\nДругие устройства остаются в системе.",
		Args:  NoArgs,
	}
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		if err := account.Logout(c.Context(), httpClient); err != nil {
			return err
		}
		return report(c, "Выход выполнен.")
	}
	return cmd
}
