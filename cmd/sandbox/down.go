package main

import "github.com/spf13/cobra"

func down(cmd *cobra.Command, _ []string) error {
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	stopped, err := stop(cmd.Context(), p.state)
	if err != nil {
		return err
	}
	status := "песочница не запущена"
	if stopped {
		status = "песочница остановлена"
	}
	return report(cmd, status)
}
