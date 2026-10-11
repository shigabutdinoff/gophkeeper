package cli

import (
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func kept(c *cobra.Command, old map[string]string, e account.Entry, login, note string) (string, string) {
	if !c.Flags().Changed("login") {
		login = old["login"]
	}
	if !c.Flags().Changed("note") {
		note = e.Note
	}
	return login, note
}
