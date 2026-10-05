package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func registerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Зарегистрироваться по email и паролю",
		Long: "Создаёт учётную запись по email и паролю. Пароль вводится только с клавиатуры.\n" +
			"Сервер не получает ни пароля, ни ключа к данным. После регистрации вход запоминается.",
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
		saved, err := account.Signup(c.Context(), httpClient, target, email, password)
		return entered(c, saved, err, "Учётная запись создана, вход выполнен.")
	}
	return cmd
}
