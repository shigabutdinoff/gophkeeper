package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Войти по email и паролю",
		Long: "Входит по email и паролю и запоминает вход на этом устройстве. Пароль вводится только с клавиатуры.\n" +
			"Ключ к данным хранится в системном хранилище секретов.",
		Args: NoArgs,
	}
	server := serverFlag(cmd)
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		target, err := resolve(*server)
		if err != nil {
			return err
		}
		email, password, err := prompt(c.InOrStdin(), c.ErrOrStderr())
		if err != nil {
			return err
		}
		saved, err := account.Login(c.Context(), httpClient, target, email, password)
		return entered(c, saved, err, "Вход выполнен.")
	}
	return cmd
}

func entered(c *cobra.Command, saved bool, err error, done string) error {
	if err != nil {
		return err
	}
	if !saved {
		fmt.Fprintln(c.ErrOrStderr(), "Системное хранилище секретов недоступно, пароль будет запрашиваться при каждой команде с данными.")
	}
	return report(c, done)
}
