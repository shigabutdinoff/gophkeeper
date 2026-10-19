package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/avast/retry-go/v5"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func copyName(e account.Entry) string {
	return e.ID + "." + record.Digest(e.Secret)
}

func sealedFile(c *cobra.Command, o online, e account.Entry) ([]byte, error) {
	if o.keys.Down != nil {
		dir, err := copyDir(o.keys.User)
		path, data := filepath.Join(dir, copyName(e)), []byte(nil)
		if err == nil {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return nil, o.keys.Down
		}
		if _, err = record.OpenFile(o.keys.Data, e.ID, data); err != nil {
			if err = os.Remove(path); err != nil {
				return nil, errors.Join(o.keys.Down, fmt.Errorf("повреждённую копию файла удалить не удалось: %w", err))
			}
			return nil, o.keys.Down
		}
		return data, nil
	}
	files := fileStore(o)
	return retry.NewWithData[[]byte](retry.Context(c.Context()), retry.Attempts(3), retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool { return errors.Is(err, account.ErrUnavailable) }),
	).Do(func() ([]byte, error) { return fetch(c.Context(), files, o.keys.User, e.ID) })
}
