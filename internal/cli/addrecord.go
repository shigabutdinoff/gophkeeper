package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func add(c *cobra.Command, s config.Settings, keys account.Secrets, k kind, login, note string) error {
	fields, err := ask(c, k, login, note, nil)
	switch {
	case err != nil:
		return err
	case k.name == "file":
		return addFile(c, s, keys, note)
	}
	msg, id, err := record.Put(keys.Data, keys.User, fields, note)
	return deliver(c, s, keys, id, msg, err)
}

var errOffline = errors.New("сервер недоступен, изменение не сохранено, попробуйте позже")

func offline(keys account.Secrets, err error) error {
	if errors.Is(err, queue.ErrDenied) && keys.Down != nil {
		return errOffline
	}
	return err
}
