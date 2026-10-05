package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Добавить логин и пароль",
		Long: "Добавляет логин и пароль с пометкой, даже когда сервер недоступен. Пароль записи вводится только с клавиатуры.\n" +
			"Логин и пароль уходят зашифрованными ключом к данным, пометка хранится на сервере открыто.",
		Args: NoArgs,
	}
	login := cmd.Flags().String("login", "", "`логин` записи")
	note := cmd.Flags().String("note", "", "`текст` открытой пометки для поиска, без секретов")
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		if *login == "" {
			return errors.New("укажите логин флагом --login")
		}
		s, err := resolve("")
		if err == nil {
			err = queue.Check(s)
		}
		if err != nil {
			return err
		}
		keys, err := account.Unlock(c.Context(), httpClient, s, func(email string) (string, error) {
			return askHidden(c, "Пароль "+email+": ")
		})
		if err != nil {
			return err
		}
		return add(c, s, keys, *login, *note)
	}
	return cmd
}
