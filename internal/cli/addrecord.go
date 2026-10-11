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
	if err != nil {
		return err
	}
	msg, id, err := record.Put(keys.Data, keys.User, fields, note)
	if err != nil {
		return err
	}
	return deliver(c, s, keys, msg, id)
}

var errOffline = errors.New("сервер недоступен, изменение не сохранено, попробуйте позже")

func deliver(c *cobra.Command, s config.Settings, keys account.Secrets, msg *nats.Msg, id string) error {
	if keys.Access == "" {
		return errOffline
	}
	err := queue.Send(c.Context(), s, msg, nats.Token(keys.Access), nats.CustomInboxPrefix(nats.InboxPrefix+keys.User))
	switch {
	case errors.Is(err, queue.ErrDenied) && keys.Down != nil:
		return errOffline
	case err != nil:
		return err
	}
	return report(c, "Изменение принято, запись "+id+".")
}
