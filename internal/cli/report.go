package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func report(c *cobra.Command, msg string) error {
	if _, err := fmt.Fprintln(c.OutOrStdout(), msg); err != nil {
		return fmt.Errorf("вывод результата: %w", err)
	}
	return nil
}
