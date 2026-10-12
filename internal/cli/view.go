package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func showRecord(c *cobra.Command, o online, id string) error {
	e, err := account.Find(c.Context(), httpClient, o.s, o.keys.Access, id)
	if err != nil {
		return err
	}
	k, f, err := unseal(o, e)
	if err != nil {
		return err
	}
	if k.name == "text" {
		return report(c, "Пометка: "+e.Note+"\nТекст:\n"+strings.TrimSuffix(f["text"], "\n"))
	}
	var out string
	if k.name == "file" {
		f["size"] = sized(f)
	}
	for _, x := range k.fields {
		out += x.label + ": " + f[x.key] + "\n"
	}
	return report(c, out+"Пометка: "+e.Note)
}
