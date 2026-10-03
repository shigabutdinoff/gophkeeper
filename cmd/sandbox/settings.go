package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const markerPrefix = "# gophkeeper-"

type settings struct {
	Server     string `yaml:"server"`
	AppKey     string `yaml:"app_key"`
	Queue      string `yaml:"queue"`
	QueueUser  string `yaml:"queue_user,omitempty"`
	QueuePass  string `yaml:"queue_password,omitempty"`
	QueueCreds string `yaml:"queue_creds_file,omitempty"`
	QueueInbox string `yaml:"queue_inbox"`
	QueueCA    string `yaml:"queue_ca,omitempty"`
}

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
	if err := writeFile(path, data); err != nil {
		return fmt.Errorf("запись файла настроек: %w", err)
	}
	return nil
}
