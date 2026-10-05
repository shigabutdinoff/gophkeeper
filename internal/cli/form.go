package cli

import (
	"errors"

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
	if errors.Is(err, huh.ErrUserAborted) {
		exit(interruptCode)
	}
	return err
}
