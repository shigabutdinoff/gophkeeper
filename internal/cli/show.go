package cli

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func showList(c *cobra.Command, o online) error {
	list, err := account.List(c.Context(), httpClient, o.s, o.keys.Access)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return report(c, "Записей нет.")
	}
	t := table.New().Headers("Тип", "Пометка", "Идентификатор").
		StyleFunc(func(int, int) lipgloss.Style { return lipgloss.NewStyle().Padding(0, 1) })
	for _, e := range list {
		k, f, err := unseal(o, e)
		if err != nil {
			k.title = "не читается"
		}
		if k.name == "file" {
			k.title += " " + f["name"] + ", " + sized(f)
		}
		t.Row(k.title, e.Note, e.ID)
	}
	if _, err = fmt.Fprintln(c.OutOrStdout(), t); err != nil {
		return fmt.Errorf("вывод результата: %w", err)
	}
	return nil
}
