package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const markerPrefix = "# gophkeeper-"

func marked(source string, data []byte) []byte {
	head := fmt.Sprintf("%s%s: файл записан командой %s, повторный запуск его заменяет\n", markerPrefix, source, source)
	return append([]byte(head), data...)
}

func httpsOnly(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("адрес Supabase %q должен начинаться с %q", url, "https://")
	}
	return nil
}

func saveSettings(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("каталог файла настроек: %w", err)
	}
	return writeFile(path, data)
}
