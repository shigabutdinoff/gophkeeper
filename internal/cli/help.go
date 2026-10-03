package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func showHelp(c *cobra.Command, _ []string) error {
	return c.Help()
}

func localizeHelpCmd(help *cobra.Command) {
	help.Use = "help [команда]"
	help.Short = "Показать справку по команде"
	help.Long = ""
	help.RunE = func(c *cobra.Command, args []string) error {
		target, rest, err := c.Root().Find(args)
		if err != nil {
			return fmt.Errorf("поиск команды для справки: %w", err)
		}
		if len(rest) > 0 {
			return unknownCommand(rest[0])
		}
		return target.Help()
	}
}
