package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func up(cmd *cobra.Command, _ []string) error {
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(p.state, 0o700); err != nil {
		return fmt.Errorf("каталог песочницы: %w", err)
	}
	if err = sandboxSecrets(p.state); err != nil {
		return fmt.Errorf("секреты песочницы: %w", err)
	}
	want, err := sandboxSettings(p.state)
	if err != nil {
		return fmt.Errorf("настройки песочницы: %w", err)
	}
	q, err := sandboxAccess(p.state)
	if err != nil {
		return err
	}
	status := "песочница уже запущена"
	if err = prepareQueue(cmd.Context(), q); err != nil {
		if !notRunning(err) {
			return err
		}
		status = "песочница запущена"
		if err = start(cmd.Context(), p.state, q); err != nil {
			return err
		}
	}
	placed, err := placeSettings(want, sandboxName, p.client)
	if err != nil {
		return err
	}
	return report(cmd, status+",", placed)
}
