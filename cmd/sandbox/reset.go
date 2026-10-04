package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"
)

func reset(cmd *cobra.Command, args []string) error {
	if err := down(cmd, args); err != nil {
		return err
	}
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	if err = os.RemoveAll(p.state); err != nil {
		return fmt.Errorf("удаление каталога песочницы: %w", err)
	}
	for _, path := range []string{p.client, asidePath(p.client, sandboxName)} {
		if err = removeMarked(path); err != nil {
			return fmt.Errorf("удаление файла настроек песочницы: %w", err)
		}
	}
	return report(cmd, "песочница сброшена")
}

func removeMarked(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || !bytes.HasPrefix(data, marked(sandboxName, nil)) {
		return err
	}
	return os.Remove(path)
}
