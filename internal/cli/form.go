package cli

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

var runForm = func(c *cobra.Command, f *huh.Form) error {
	return f.WithInput(c.InOrStdin()).WithOutput(c.ErrOrStderr()).Run()
}

func fill(c *cobra.Command, inputs ...huh.Field) error {
	if err := terminal(c.InOrStdin()); err != nil {
		return err
	}
	f := huh.NewForm(huh.NewGroup(inputs...)).WithShowHelp(false).WithViewHook(func(v tea.View) tea.View {
		v.DisableBracketedPasteMode = true
		return v
	})
	err := runForm(c, f)
	if err == nil && f.State != huh.StateCompleted {
		err = huh.ErrUserAborted
	}
	return err
}

func askForm(c *cobra.Command, k kind, vals []string) error {
	inputs := make([]huh.Field, len(k.fields))
	for i, f := range k.fields {
		inputs[i] = huh.NewInput().Title(f.label).EchoMode(f.echo).Value(&vals[i])
	}
	return fill(c, inputs...)
}
