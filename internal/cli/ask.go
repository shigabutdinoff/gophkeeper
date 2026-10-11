package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func ask(c *cobra.Command, k kind, login, note string, old map[string]string) (fields map[string]string, err error) {
	if k.name == "" {
		fields = map[string]string{"login": login}
		fields["password"], err = askHidden(c, "Пароль записи")
		if err == nil && fields["password"] == "" {
			err = errors.New("пароль записи не может быть пустым")
		}
	} else {
		fields, err = askFields(c, k, old)
	}
	if err == nil && note != "" {
		c.PrintErrln("Пометка хранится на сервере без шифрования, секретов в ней быть не должно.")
	}
	return fields, err
}
