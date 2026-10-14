package cli

import (
	"cmp"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func askFields(c *cobra.Command, k kind, old map[string]string) (map[string]string, error) {
	vals, ask := make([]string, len(k.fields)), askForm
	if k.name == "text" {
		ask = askText
	}
	if err := ask(c, k, vals); err != nil {
		return nil, err
	}
	got := map[string]string{"type": k.name}
	for i, f := range k.fields {
		if got[f.key] = cmp.Or(vals[i], old[f.key]); got[f.key] == "" && f.need {
			return nil, fmt.Errorf("%s не может быть пустым", strings.ToLower(f.label))
		}
	}
	return got, nil
}

func askText(c *cobra.Command, k kind, vals []string) error {
	if err := terminal(c.InOrStdin()); err == nil {
		_, _ = fmt.Fprintln(c.ErrOrStderr(), k.fields[0].label+", конец ввода Ctrl-D:")
	}
	text, err := io.ReadAll(c.InOrStdin())
	if err != nil {
		return fmt.Errorf("чтение текста: %w", err)
	}
	vals[0] = string(text)
	return nil
}
