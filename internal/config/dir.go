package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dir возвращает каталог gophkeeper внутри XDG_CONFIG_HOME, если это
// абсолютный путь, а иначе внутри .config в домашнем каталоге.
func Dir() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("каталог настроек: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "gophkeeper"), nil
}
