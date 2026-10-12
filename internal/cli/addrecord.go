package cli

import (
	"errors"

	"github.com/nats-io/nats.go"
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

func deliver(c *cobra.Command, s config.Settings, keys account.Secrets, id string, msg *nats.Msg, err error) error {
	switch {
	case err != nil:
		return err
	case keys.Access == "":
		return errOffline
	}
	err = queue.Send(c.Context(), s, msg, nats.Token(keys.Access), nats.CustomInboxPrefix(nats.InboxPrefix+keys.User))
	if err = offline(keys, err); err != nil {
		return err
	}
	return report(c, "Изменение принято, запись "+id+".")
}
