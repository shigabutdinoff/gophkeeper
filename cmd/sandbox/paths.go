package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const settingsFile = "config.yaml"

type sandboxPaths struct {
	state, client string
}

func paths(name string) (sandboxPaths, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return sandboxPaths{}, fmt.Errorf("каталог настроек: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	base := filepath.Join(dir, "gophkeeper")
	return sandboxPaths{filepath.Join(base, name), filepath.Join(base, settingsFile)}, nil
}
