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

// Settings содержит настройки клиента для связи с сервером.
type Settings struct {
	// Server задаёт адрес сервера.
	Server string `yaml:"server"`
	// AppKey задаёт открытый ключ приложения на сервере.
	AppKey string `yaml:"app_key"`
}

// Load читает настройки из файла path. Если файла нет, возвращает пустые
// настройки без ошибки.
func Load(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("чтение файла настроек: %w", err)
	}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("разбор файла настроек %q: %w", path, err)
	}
	return s, nil
}
