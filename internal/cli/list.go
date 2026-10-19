package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [идентификатор]",
		Short: "Показать записи или одну запись",
		Long: "Без идентификатора показывает тип, пометку и идентификатор каждой записи, без их содержимого.\n" +
			"С идентификатором показывает поля и пометку записи. Записи читаются с сервера, неотправленные изменения\n" +
			"читаются с устройства и помечаются «не отправлено». Без сервера видны только неотправленные записи.\n" +
			"С --note показывает только записи, в пометке которых есть подстрока, без учёта регистра.",
		Args: maxOneArg,
	}
	note := cmd.Flags().String("note", "", "`подстрока` пометки, чтобы показать только подходящие записи")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if *note != "" && len(args) > 0 {
			return errors.New("флаг --note нельзя указывать вместе с идентификатором")
		}
		o, err := connect(c)
		switch {
		case err != nil:
		case len(args) == 0:
			err = showList(c, o, *note)
		default:
			err = showRecord(c, o, args[0])
		}
		return account.Reason(err)
	}
	return cmd
}
