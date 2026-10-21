package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"uuid"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const copyList = "list"

func copyDir(user string) (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	if u, err := uuid.Parse(user); err != nil || u.String() != user {
		return "", errors.New("идентификатор пользователя не UUID")
	}
	return filepath.Join(dir, "copy", user), nil
}

func readCopy(c *cobra.Command, o online) ([]account.Entry, error) {
	dir, err := copyDir(o.keys.User)
	var sealed, raw []byte
	if err == nil {
		sealed, err = os.ReadFile(filepath.Join(dir, copyList))
	}
	if err == nil {
		raw, err = record.OpenFile(o.keys.Data, copyList, sealed)
	}
	var list []account.Entry
	if err == nil {
		err = json.Unmarshal(raw, &list)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.PrintErrln("копию записей на устройстве прочитать не удалось:", err)
	}
	return list, err
}
