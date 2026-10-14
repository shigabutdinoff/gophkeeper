package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func editCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <идентификатор>",
		Short: "Изменить запись",
		Long: "Меняет пароль записи, логин и пометку остаются прежними, если не заданы флагами. Пароль вводится только с клавиатуры.\n" +
			"У текста и карты пустой ввод поля оставляет прежнее значение, у файла меняется только пометка.\n" +
			"Прежние значения читаются с сервера, без него изменение не принимается.",
		Args: oneArg,
	}
	login := cmd.Flags().String("login", "", "новый `логин` записи")
	note := cmd.Flags().String("note", "", "новый `текст` открытой пометки, без секретов")
	cmd.Flags().String("file", "", "`путь` нового файла, замена файла не поддерживается")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if c.Flags().Changed("file") {
			return errors.New("чтобы заменить файл, удалите запись и добавьте файл заново")
		}
		o, e, err := lookup(c, args[0], false)
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
		return deliver(c, o.s, o.keys, e.ID, msg, err)
	}
	return cmd
}
