package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaceSettings(t *testing.T) {
	dir := t.TempDir()
	clientPath := filepath.Join(dir, "gophkeeper", "config.yaml")
	want := marked("sandbox", []byte("песочница\n"))
	maskDir := func(msg string) string { return strings.ReplaceAll(msg, dir, "<dir>") }

	msg, err := placeSettings(want, "sandbox", clientPath)
	require.NoError(t, err, "файл клиента не записан")
	got, err := os.ReadFile(clientPath)
	require.NoError(t, err, "файл клиента не прочитан")
	assert.Equal(t, string(want), string(got), "файл клиента не совпал с настройками песочницы")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(clientPath)
		require.NoError(t, err, "файл клиента не найден")
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "файл клиента доступен другим")
	}
	snaps.MatchSnapshot(t, maskDir(msg.String()))

	msg, err = placeSettings(want, "sandbox", clientPath)
	require.NoError(t, err, "совпадающий файл клиента не проверен")
	snaps.MatchSnapshot(t, maskDir(msg.String()))

	require.NoError(t, os.WriteFile(clientPath, []byte("личное\n"), 0o600), "личный файл не записан")
	msg, err = placeSettings(want, "sandbox", clientPath)
	require.NoError(t, err, "личный файл клиента не проверен")
	got, err = os.ReadFile(clientPath)
	require.NoError(t, err, "личный файл не прочитан")
	assert.Equal(t, "личное\n", string(got), "личный файл клиента изменился")
	aside, err := os.ReadFile(filepath.Join(dir, "gophkeeper", "config.sandbox.yaml"))
	require.NoError(t, err, "настройки рядом не записаны")
	assert.Equal(t, string(want), string(aside), "рядом записаны не те настройки")
	snaps.MatchSnapshot(t, maskDir(msg.String()))
}

func TestPlaceSettingsReplacesMarked(t *testing.T) {
	clientPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(clientPath, marked("sandbox", []byte("песочница\n")), 0o600), "файл песочницы не записан")
	want := marked("cloud", []byte("облако\n"))
	_, err := placeSettings(want, "cloud", clientPath)
	require.NoError(t, err, "файл песочницы не заменён")
	got, err := os.ReadFile(clientPath)
	require.NoError(t, err, "файл клиента не прочитан")
	assert.Equal(t, string(want), string(got), "помеченный файл не заменён")
}

func TestWriteFileCleansTmp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.Mkdir(path, 0o700), "каталог на месте файла не создан")
	require.Error(t, writeFile(path, []byte("{}")), "файл записан поверх каталога")
	assert.NoFileExists(t, path+".tmp", "после сбоя остался временный файл")
}
