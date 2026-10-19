package cli

import (
	"uuid"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func copyEntry(o online, id string) (account.Entry, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return account.Entry{}, account.ErrNotFound
	}
	list, err := readCopy(o)
	if err != nil {
		return account.Entry{}, o.keys.Down
	}
	for _, e := range list {
		if e.ID == u.String() {
			return e, nil
		}
	}
	return account.Entry{}, o.keys.Down
}
