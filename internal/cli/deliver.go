package cli

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

func deliver(c *cobra.Command, s config.Settings, keys account.Secrets, id string, msg *nats.Msg, err error) error {
	if err != nil {
		return err
	}
	box, err := queue.Open(c.Context(), s, keys.User, keys.Data, keys.Access)
	if err != nil {
		return err
	}
	defer box.Close()
	if err = box.Put(c.Context(), msg); err != nil {
		return err
	}
	err = box.Drain(c.Context())
	switch {
	case err == nil:
		return report(c, "Изменение принято, запись "+id+".")
	case errors.Is(err, queue.ErrDenied) && keys.Down == nil && keys.Access != "":
		return fmt.Errorf("%w, изменение сохранено на устройстве и уйдёт после повторного входа", err)
	}
	return report(c, "Изменение сохранено на устройстве и будет отправлено позже, запись "+id+".")
}
