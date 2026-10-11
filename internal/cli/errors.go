package cli

import (
	"errors"
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

func maxOneArg(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("лишний аргумент %q", args[1])
	}
	return nil
}

func oneArg(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("укажите идентификатор записи")
	}
	return maxOneArg(c, args)
}
