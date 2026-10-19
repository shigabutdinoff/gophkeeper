package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/google/renameio/v2/maybe"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
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
	sealed, err := sealedFile(c, o, e)
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
