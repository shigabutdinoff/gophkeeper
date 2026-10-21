package cli

import (
	"fmt"
	"unicode/utf8"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

const minPassword = 8

var isTerminal = term.IsTerminal

func prompt(c *cobra.Command) (email, password string, err error) {
	err = fill(c, huh.NewInput().Title("Email").Value(&email),
		huh.NewInput().Title("Пароль").EchoMode(huh.EchoModePassword).Value(&password))
	if err == nil && utf8.RuneCountInString(password) < minPassword {
		err = fmt.Errorf("пароль должен быть не короче %d символов", minPassword)
	}
	return email, password, err
}
