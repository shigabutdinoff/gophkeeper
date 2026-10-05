package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func add(c *cobra.Command, s config.Settings, keys account.Secrets, login, note string) error {
	password, err := askHidden(c, "Пароль записи: ")
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("пароль записи не может быть пустым")
	}
	if note != "" {
		fmt.Fprintln(c.ErrOrStderr(), "Пометка хранится на сервере без шифрования, секретов в ней быть не должно.")
	}
	msg, id, err := record.Put(keys.Data, keys.Sign, login, password, note)
	if err == nil {
		err = queue.Send(c.Context(), s, msg)
	}
	if err != nil {
		return err
	}
	return report(c, "Изменение принято, запись "+id+".")
}
