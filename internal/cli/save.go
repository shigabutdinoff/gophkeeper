package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/avast/retry-go/v5"
	"github.com/dustin/go-humanize"
	"github.com/google/renameio/v2/maybe"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

var errDamaged = errors.New("файл записи повреждён или пропал на сервере, удалите запись и добавьте файл заново")

func sized(f map[string]string) string {
	n, _ := strconv.ParseUint(f["size"], 10, 64)
	return humanize.Bytes(n)
}

func save(c *cobra.Command, o online, e account.Entry, path string) error {
	k, f, err := unseal(o, e)
	switch {
	case err != nil:
		return err
	case k.name != "file":
		return fmt.Errorf("в записи %s нет файла, её поля показывает gophkeeper list", e.ID)
	case o.s.Region == "":
		return errNoRegion
	}
	files := storage.New(fileClient(), o.s.Server, o.s.Region, o.s.AppKey, o.keys.Access)
	sealed, err := retry.NewWithData[[]byte](retry.Context(c.Context()), retry.Attempts(3), retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool { return errors.Is(err, account.ErrUnavailable) }),
	).Do(func() ([]byte, error) { return fetch(c.Context(), files, o.keys.User, e.ID) })
	if err != nil {
		return err
	}
	plain, err := record.OpenFile(o.keys.Data, e.ID, sealed)
	if err != nil || strconv.Itoa(len(plain)) != f["size"] {
		return errDamaged
	}
	if err = maybe.WriteFile(path, plain, 0o600); err != nil {
		return fmt.Errorf("запись файла %s: %w", path, err)
	}
	return report(c, "Файл сохранён: "+path)
}
