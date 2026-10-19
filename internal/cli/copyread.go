package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"uuid"

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

func readCopy(o online) ([]account.Entry, error) {
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
	return list, err
}
