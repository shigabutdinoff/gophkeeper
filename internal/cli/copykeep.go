package cli

import (
	"errors"
	"os"
	"path/filepath"
	"uuid"

	"github.com/google/renameio/v2/maybe"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func keepFiles(c *cobra.Command, o online, list []account.Entry, dir string, have map[string]bool) error {
	files := fileStore(o)
	var errs []error
	for _, e := range list {
		u, err := uuid.Parse(e.ID)
		if err != nil || u.String() != e.ID {
			continue
		}
		if k, _, err := unseal(o, e); err != nil || k.name != "file" {
			continue
		}
		name := copyName(e)
		path := filepath.Join(dir, name)
		if _, err = os.Stat(path); err == nil {
			have[name] = true
			continue
		}
		data, err := fetch(c.Context(), files, o.keys.User, e.ID)
		if err == nil {
			err = maybe.WriteFile(path, data, 0o600)
		}
		have[name] = err == nil
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
