package cli

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func showList(c *cobra.Command, o online) error {
	list, err := []account.Entry(nil), o.keys.Down
	if err == nil {
		list, err = account.List(c.Context(), httpClient, o.s, o.keys.Access)
	}
	head := ""
	switch list = merge(list, o.left); {
	case err != nil && (o.keys.Down == nil || len(list) == 0):
		return err
	case err != nil:
		head = "сервер недоступен, показаны только неотправленные записи\n"
	case len(list) == 0:
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
		if _, ok := o.left[e.ID]; ok {
			k.title += ", " + unsentMark
		}
		t.Row(k.title, e.Note, e.ID)
	}
	return report(c, head+t.String())
}
