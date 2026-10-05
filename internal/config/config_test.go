package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	dir, err := Dir()
	require.NoError(t, err, "каталог настроек не найден")
	assert.Equal(t, filepath.Join(xdg, "gophkeeper"), dir, "абсолютный XDG_CONFIG_HOME пропущен")
	home, err := os.UserHomeDir()
	require.NoError(t, err, "домашний каталог не найден")
	t.Setenv("XDG_CONFIG_HOME", "relative")
	dir, err = Dir()
	require.NoError(t, err, "каталог настроек не найден")
	assert.Equal(t, filepath.Join(home, ".config", "gophkeeper"), dir, "относительный XDG_CONFIG_HOME не пропущен")
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := Load[Settings](filepath.Join(dir, "missing.yaml"))
	require.NoError(t, err, "отсутствие файла считается ошибкой")
	assert.Zero(t, s, "без файла настройки не пустые")
	path := filepath.Join(dir, File)
	require.NoError(t, os.WriteFile(path, []byte("server: https://x\napp_key: k\nqueue: tls://q\nqueue_user: u\nqueue_password: p\nqueue_creds_file: c\nqueue_inbox: i\nqueue_ca: ca\n"), 0o600), "файл не записан")
	s, err = Load[Settings](path)
	require.NoError(t, err, "файл настроек не прочитан")
	assert.Equal(t, Settings{Server: "https://x", AppKey: "k", Queue: "tls://q", QueueUser: "u", QueuePass: "p",
		QueueCreds: "c", QueueInbox: "i", QueueCA: "ca"}, s, "настройки прочитаны неверно")
	require.NoError(t, os.WriteFile(path, []byte("server: [\n"), 0o600), "файл не записан")
	_, err = Load[Settings](path)
	require.Error(t, err, "битый файл прочитан без ошибки")
	_, err = Load[Settings](dir)
	require.Error(t, err, "каталог прочитан как файл")
}
