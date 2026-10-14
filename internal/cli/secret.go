package cli

import (
	"errors"
	"io"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

func terminal(in io.Reader) error {
	f, ok := in.(interface{ Fd() uintptr })
	if !ok || !isTerminal(f.Fd()) {
		return errors.New("пароль вводится только с клавиатуры")
	}
	return nil
}

func askHidden(c *cobra.Command, label string) (pass string, err error) {
	return pass, fill(c, huh.NewInput().Title(label).EchoMode(huh.EchoModePassword).Value(&pass))
}
