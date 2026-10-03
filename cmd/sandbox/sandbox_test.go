package main

import (
	"crypto/tls"
	"crypto/x509"
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

func sandboxDirs(t *testing.T) (state, mount string) {
	t.Setenv(supabaseURL, "https://project.supabase.co")
	t.Setenv(supabaseKey, "ЗАГЛУШКА КЛЮЧА")
	t.Setenv("CONFIG_DIR", "<host>")
	return t.TempDir(), t.TempDir()
}

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

func TestPrepareSandbox(t *testing.T) {
	state, mount := sandboxDirs(t)
	first, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "песочница не подготовлена")
	files := readFiles(t, state)
	again, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "повторный запуск не прошёл")
	assert.Equal(t, files, readFiles(t, state), "повторный запуск перевыпустил сертификаты или секреты")
	client, err := os.ReadFile(filepath.Join(mount, "gophkeeper", settingsFile))
	require.NoError(t, err, "файл настроек клиента не записан")
	assert.NotEqual(t, files["admin"], files["client"], "пароли администратора и клиента совпали")
	secrets := strings.NewReplacer(files["admin"], "<admin>", files["client"], "<client>")
	ca := regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----.*-----END CERTIFICATE-----`)
	masked := ca.ReplaceAllString(secrets.Replace(string(client)), "<ca>")
	snaps.MatchSnapshot(t, slices.Sorted(maps.Keys(files)), first.String(), again.String(), masked,
		secrets.Replace(files["passwords.conf"]))
}

func TestSandboxCerts(t *testing.T) {
	state := t.TempDir()
	_, err := sandboxSecrets(state)
	require.NoError(t, err, "секреты песочницы не созданы")
	pair, err := tls.LoadX509KeyPair(filepath.Join(state, "cert.pem"), filepath.Join(state, "key.pem"))
	require.NoError(t, err, "сертификат и ключ песочницы не подходят друг другу")
	ca, err := os.ReadFile(filepath.Join(state, "ca.pem"))
	require.NoError(t, err, "CA песочницы не прочитан")
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca), "CA песочницы не разобран")
	for _, name := range []string{"127.0.0.1", "localhost", "::1", "nats"} {
		_, err := pair.Leaf.Verify(x509.VerifyOptions{DNSName: name, Roots: roots})
		assert.NoError(t, err, "клиент не доверяет сертификату песочницы для %q", name)
	}
	snaps.MatchSnapshot(t, pair.Leaf.NotAfter.Sub(pair.Leaf.NotBefore).String())
}

func TestPrepareSandboxNoSupabase(t *testing.T) {
	for _, name := range []string{supabaseURL, supabaseKey} {
		t.Run(name, func(t *testing.T) {
			state, mount := sandboxDirs(t)
			t.Setenv(name, "")
			_, err := prepareSandbox(state, mount, ".")
			require.Error(t, err, "первый запуск без %s прошёл", name)
			assert.NoDirExists(t, filepath.Join(mount, "gophkeeper"), "настройки записаны без %s", name)
			snaps.MatchSnapshot(t, err.Error())
		})
	}
	state, mount := sandboxDirs(t)
	_, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "запуск с Supabase не прошёл")
	t.Setenv(supabaseURL, "")
	_, err = prepareSandbox(state, mount, ".")
	assert.Error(t, err, "повторный запуск прошёл без Supabase")
}

func TestPrepareSandboxNewKey(t *testing.T) {
	state, mount := sandboxDirs(t)
	_, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "песочница не подготовлена")
	t.Setenv(supabaseKey, "НОВЫЙ КЛЮЧ")
	p, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "запуск с новым ключом не прошёл")
	client, err := os.ReadFile(filepath.Join(mount, "gophkeeper", settingsFile))
	require.NoError(t, err, "файл настроек клиента не прочитан")
	assert.Contains(t, string(client), "НОВЫЙ КЛЮЧ", "запуск оставил прежний ключ Supabase")
	snaps.MatchSnapshot(t, p.String())
}

func TestPrepareSandboxPlainURL(t *testing.T) {
	state, mount := sandboxDirs(t)
	t.Setenv(supabaseURL, "http://project.supabase.co")
	_, err := prepareSandbox(state, mount, ".")
	require.Error(t, err, "песочница приняла адрес Supabase без HTTPS")
	assert.NoDirExists(t, filepath.Join(mount, "gophkeeper"), "настройки записаны с адресом без HTTPS")
	snaps.MatchSnapshot(t, err.Error())
}

func TestPrepareSandboxForeign(t *testing.T) {
	state, mount := sandboxDirs(t)
	clientPath := filepath.Join(mount, "gophkeeper", settingsFile)
	require.NoError(t, os.Mkdir(filepath.Dir(clientPath), 0o700), "каталог настроек не создан")
	require.NoError(t, os.WriteFile(clientPath, []byte("server: https://example.com\n"), 0o600), "личный файл не записан")
	p, err := prepareSandbox(state, mount, ".")
	require.NoError(t, err, "песочница не подготовлена рядом с личным файлом")
	assert.FileExists(t, filepath.Join(mount, "gophkeeper", "config.sandbox.yaml"), "настройки песочницы не легли рядом")
	snaps.MatchSnapshot(t, p.String())
}

func TestPrepareSandboxFailures(t *testing.T) {
	obstacles := []string{"sandbox/cert.pem/", "sandbox/ca.pem/", "sandbox/admin/", "sandbox/client/", "sandbox/passwords.conf/",
		"sandbox/cert.pem", "config/gophkeeper/config.yaml/"}
	for _, obstacle := range obstacles {
		t.Run(obstacle, func(t *testing.T) {
			sandboxDirs(t)
			root := t.TempDir()
			path := filepath.Join(root, obstacle)
			require.NoError(t, os.MkdirAll(filepath.Join(root, "sandbox"), 0o700), "каталог песочницы не создан")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700), "каталог помехи не создан")
			if strings.HasSuffix(obstacle, "/") {
				require.NoError(t, os.Mkdir(path, 0o700), "помеха не создана")
			} else {
				require.NoError(t, os.WriteFile(path, []byte("занято"), 0o600), "помеха не создана")
			}
			_, err := prepareSandbox(filepath.Join(root, "sandbox"), filepath.Join(root, "config"), ".")
			require.Error(t, err, "запуск с помехой %q прошёл", obstacle)
			step, _, _ := strings.Cut(err.Error(), ": ")
			snaps.MatchSnapshot(t, filepath.ToSlash(strings.ReplaceAll(step, root, "<root>")))
		})
	}
}

func TestChownLike(t *testing.T) {
	dir := t.TempDir()
	assert.Error(t, chownLike([]string{filepath.Join(dir, "нет")}, dir), "владелец взят у несуществующего каталога")
	if runtime.GOOS != "windows" {
		assert.Error(t, chownLike([]string{dir}, filepath.Join(dir, "нет")), "владелец назначен несуществующему файлу")
		assert.NoError(t, chownLike([]string{"/", dir}, dir), "каталог root не уступил владельцу исходников")
	}
}
