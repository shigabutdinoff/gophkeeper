package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <идентификатор>",
		Short: "Удалить запись",
		Long:  "Удаляет запись. Когда сервер недоступен, удаление уходит в очередь без проверки.",
		Args:  oneArg,
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		o, e, err := lookup(c, args[0], true)
		if err != nil {
			return account.Reason(err)
		}
		msg, err := record.Delete(o.keys.User, e.ID)
		return deliver(c, o.s, o.keys, e.ID, msg, err)
	}
	return cmd
}
