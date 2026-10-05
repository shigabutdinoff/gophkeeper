package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"go.yaml.in/yaml/v3"
)

// File задаёт имя файла настроек клиента в каталоге Dir.
const File = "config.yaml"

// Load читает YAML из файла path в значение типа T. Если файла нет,
// возвращает нулевое значение без ошибки.
func Load[T any](path string) (T, error) {
	var s T
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("чтение файла настроек: %w", err)
	}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return *new(T), fmt.Errorf("разбор файла настроек %q: %w", path, err)
	}
	return s, nil
}
