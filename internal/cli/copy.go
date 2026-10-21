package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func keepCopy(c *cobra.Command, o online, list []account.Entry) error {
	dir, err := copyDir(o.keys.User)
	if err == nil {
		err = os.MkdirAll(dir, 0o700)
	}
	var raw, sealed []byte
	if err == nil {
		raw, err = json.Marshal(list)
	}
	if err == nil {
		sealed, err = record.SealFile(o.keys.Data, copyList, raw)
	}
	if err == nil {
		err = maybe.WriteFile(filepath.Join(dir, copyList), sealed, 0o600)
	}
	if err != nil {
		return fmt.Errorf("копию записей на устройстве обновить не удалось: %w", err)
	}
	have := map[string]bool{copyList: true}
	if o.s.Region != "" {
		err = keepFiles(c, o, list, dir, have)
	}
	if err = errors.Join(err, pruneCopy(dir, have)); err != nil {
		return fmt.Errorf("копию файлов на устройстве обновить не удалось: %w", err)
	}
	return nil
}
