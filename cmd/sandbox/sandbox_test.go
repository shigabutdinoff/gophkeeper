package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFiles(t *testing.T, dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "каталог песочницы не прочитан")
	files := map[string]string{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err, "файл песочницы %q не прочитан", e.Name())
		files[e.Name()] = string(data)
		if info, err := e.Info(); err == nil && runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "файл песочницы %q доступен другим", e.Name())
		}
	}
	return files
}

func TestSandboxSecrets(t *testing.T) {
	state := t.TempDir()
	require.NoError(t, sandboxSecrets(state), "секреты песочницы не созданы")
	files := readFiles(t, state)
	require.NoError(t, sandboxSecrets(state), "повторный запуск не прошёл")
	assert.Equal(t, files, readFiles(t, state), "повторный запуск перевыпустил сертификаты или секреты")
	assert.NotEqual(t, files["admin"], files["client"], "пароли администратора и клиента совпали")
	snaps.MatchSnapshot(t, slices.Sorted(maps.Keys(files)))
}

func TestSandboxSettings(t *testing.T) {
	state := t.TempDir()
	require.NoError(t, sandboxSecrets(state), "секреты песочницы не созданы")
	client, err := readPass(state, clientUser)
	require.NoError(t, err, "пароль клиента не прочитан")
	for _, c := range []struct {
		name, url, key, state string
		missing               bool
	}{
		{"пустое окружение", "", "", state, false},
		{"адрес без HTTPS", "http://project.supabase.co", "ЗАГЛУШКА КЛЮЧА", state, false},
		{"нет CA", "https://project.supabase.co", "ЗАГЛУШКА КЛЮЧА", t.TempDir(), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(supabaseURL, c.url)
			t.Setenv(supabaseKey, c.key)
			_, serr := sandboxSettings(c.state)
			require.Error(t, serr, "настройки собраны")
			if c.missing {
				require.ErrorIs(t, serr, fs.ErrNotExist, "настройки собраны без CA")
				return
			}
			snaps.MatchSnapshot(t, serr.Error())
		})
	}
	t.Run("готово", func(t *testing.T) {
		t.Setenv(supabaseURL, "https://project.supabase.co")
		t.Setenv(supabaseKey, "ЗАГЛУШКА КЛЮЧА")
		data, serr := sandboxSettings(state)
		require.NoError(t, serr, "настройки песочницы не собраны")
		ca := regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----.*-----END CERTIFICATE-----`)
		snaps.MatchSnapshot(t, ca.ReplaceAllString(strings.ReplaceAll(string(data), string(client), "<client>"), "<ca>"))
	})
}

func TestSandboxCerts(t *testing.T) {
	state := t.TempDir()
	require.NoError(t, sandboxSecrets(state), "секреты песочницы не созданы")
	pair, err := tls.LoadX509KeyPair(filepath.Join(state, certFile), filepath.Join(state, keyFile))
	require.NoError(t, err, "сертификат и ключ песочницы не подходят друг другу")
	ca, err := os.ReadFile(filepath.Join(state, caFile))
	require.NoError(t, err, "CA песочницы не прочитан")
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca), "CA песочницы не разобран")
	for _, name := range []string{"127.0.0.1", "localhost", "::1"} {
		_, err := pair.Leaf.Verify(x509.VerifyOptions{DNSName: name, Roots: roots})
		assert.NoError(t, err, "клиент не доверяет сертификату песочницы для %q", name)
	}
	snaps.MatchSnapshot(t, pair.Leaf.NotAfter.Sub(pair.Leaf.NotBefore).String())
}
