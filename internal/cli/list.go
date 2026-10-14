package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [идентификатор]",
		Short: "Показать записи или одну запись",
		Long: "Без идентификатора показывает тип, пометку и идентификатор каждой записи, без их содержимого.\n" +
			"С идентификатором показывает поля и пометку записи. Записи читаются с сервера, неотправленные изменения\n" +
			"читаются с устройства и помечаются «не отправлено». Без сервера видны только неотправленные записи.",
		Args: maxOneArg,
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		o, err := connect(c)
		switch {
		case err != nil:
		case len(args) == 0:
			err = showList(c, o)
		default:
			err = showRecord(c, o, args[0])
		}
		return account.Reason(err)
	}
	return cmd
}
