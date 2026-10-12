package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <идентификатор> <путь>",
		Short: "Сохранить файл записи на диск",
		Long: "Скачивает файл записи, расшифровывает его и сохраняет по пути доступным только владельцу.\n" +
			"Файл, который уже лежит по этому пути, не заменяется.",
		Args: twoArgs,
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if _, err := os.Lstat(args[1]); err == nil {
			return fmt.Errorf("по пути %s уже есть файл, укажите другой путь", args[1])
		}
		o, e, err := lookup(c, args[0], false)
		if err != nil {
			return account.Reason(err)
		}
		return save(c, o, e, args[1])
	}
	return cmd
}

func twoArgs(_ *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return errors.New("укажите идентификатор записи")
	case 1:
		return errors.New("укажите путь, по которому сохранить файл")
	case 2:
		return nil
	}
	return fmt.Errorf("лишний аргумент %q", args[2])
}
