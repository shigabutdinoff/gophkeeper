package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func copyEntry(c *cobra.Command, o online, id string) (account.Entry, error) {
	list, err := readCopy(c, o)
	i := slices.IndexFunc(list, func(e account.Entry) bool { return e.ID == id })
	if err != nil || i < 0 {
		return account.Entry{}, o.keys.Down
	}
	return list[i], nil
}
