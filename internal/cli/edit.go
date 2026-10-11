package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func editCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <идентификатор>",
		Short: "Изменить запись",
		Long: "Меняет пароль записи, логин и пометку остаются прежними, если не заданы флагами. Пароль вводится только с клавиатуры.\n" +
			"У текста и карты пустой ввод поля оставляет прежнее значение, тип записи не меняется.\n" +
			"Прежние значения читаются с сервера, без него изменение не принимается.",
		Args: oneArg,
	}
	login := cmd.Flags().String("login", "", "новый `логин` записи")
	note := cmd.Flags().String("note", "", "новый `текст` открытой пометки, без секретов")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		o, e, err := lookup(c, args[0], false)
		defer o.stop()
		if err != nil {
			return account.Reason(err)
		}
		k, old, err := unseal(o, e)
		if err != nil {
			return err
		}
		user, text := kept(c, old, e, *login, *note)
		fields, err := ask(c, k, user, text, old)
		if err != nil {
			return err
		}
		msg, err := record.Edit(o.keys.Data, o.keys.User, e.ID, fields, text)
		if err != nil {
			return err
		}
		return deliver(c, o.s, o.keys, msg, e.ID)
	}
	return cmd
}
