package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func report(cmd *cobra.Command, a ...any) error {
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), a...); err != nil {
		return fmt.Errorf("вывод результата: %w", err)
	}
	return nil
}
