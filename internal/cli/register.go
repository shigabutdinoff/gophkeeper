package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func registerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Зарегистрироваться по email и паролю",
		Long: "Создаёт учётную запись по email и паролю. Пароль вводится только с клавиатуры.\n" +
			"Сервер не получает ни пароля, ни ключа к данным.",
		Args: NoArgs,
	}
	server := cmd.Flags().String("server", "", "`адрес` сервера, если не задана "+serverEnv+", важнее файла настроек")
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		target, err := resolve(*server)
		if err != nil {
			return err
		}
		email, password, err := prompt(c.InOrStdin(), c.ErrOrStderr())
		if err != nil {
			return err
		}
		if err = account.Signup(c.Context(), httpClient, target, email, password); err != nil {
			return err
		}
		_, err = fmt.Fprintln(c.OutOrStdout(), "Учётная запись создана.")
		return err
	}
	return cmd
}
