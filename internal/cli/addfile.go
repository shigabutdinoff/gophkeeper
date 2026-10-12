package cli

import (
	"path/filepath"
	"strconv"
	"uuid"

	"charm.land/huh/v2"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func addFile(c *cobra.Command, s config.Settings, keys account.Secrets, note string) error {
	var path string
	if s.Region == "" {
		return errNoRegion
	}
	if err := fill(c, huh.NewFilePicker().Title("Файл").Picking(true).Value(&path)); err != nil {
		return err
	}
	data, err := load(path)
	if err != nil {
		return err
	}
	id := uuid.NewV7().String()
	sealed, err := record.SealFile(keys.Data, id, data)
	if err != nil {
		return err
	}
	fields := map[string]string{"type": "file", "name": filepath.Base(path), "size": strconv.Itoa(len(data))}
	msg, err := record.PutFile(keys.Data, keys.User, id, fields, note, sealed)
	if err != nil {
		return err
	}
	err = queue.Upload(c.Context(), s, keys.User, id, sealed, nats.Token(keys.Access), nats.CustomInboxPrefix(nats.InboxPrefix+keys.User))
	return deliver(c, s, keys, id, msg, offline(keys, err))
}
