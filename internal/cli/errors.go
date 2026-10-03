package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func unknownCommand(name string) error {
	return fmt.Errorf("неизвестная команда %q", name)
}

func noSubcommand(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return unknownCommand(args[0])
	}
	return nil
}

// NoArgs отклоняет любой аргумент команды русской ошибкой. Подходит для
// поля Args команды cobra.
func NoArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("лишний аргумент %q", args[0])
	}
	return nil
}
