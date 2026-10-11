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
		Short: "Добавить запись",
		Long: "Добавляет логин и пароль, текст или банковскую карту с пометкой, даже когда сервер недоступен.\n" +
			"Пароль, номер и код карты вводятся только с клавиатуры, текст читается из stdin до конца ввода.\n" +
			"Поля записи уходят зашифрованными ключом к данным, пометка хранится на сервере открыто.",
		Args: NoArgs,
	}
	login := cmd.Flags().String("login", "", "`логин` записи")
	note := cmd.Flags().String("note", "", "`текст` открытой пометки для поиска, без секретов")
	typ := cmd.Flags().String("type", "", "`тип` записи text или card, без флага логин и пароль")
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		k, err := kindOf(*typ)
		if err != nil {
			return err
		}
		if k.name == "" && *login == "" {
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
			return askHidden(c, "Пароль "+email)
		})
		if err != nil {
			return err
		}
		return add(c, s, keys, k, *login, *note)
	}
	return cmd
}
