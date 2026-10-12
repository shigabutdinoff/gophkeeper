package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func loadWith(t *testing.T, env map[string]string, args ...string) string {
	t.Helper()
	var s settings
	fs := pflag.NewFlagSet("worker", pflag.ContinueOnError)
	flags := addFlags(fs, &s)
	for _, f := range flags {
		t.Setenv(f.EnvNames[0], "")
	}
	t.Setenv("CONFIG", "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	require.NoError(t, fs.Parse(args), "флаги не разобраны")
	if err := load(fs, &s, flags); err != nil {
		return "ошибка: " + err.Error()
	}
	return s.Queue + "|" + s.Server + " " + s.ServerKey + " " + s.AccessSeed + " " + s.Region + " " + s.AppKey
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	yaml, json := filepath.Join(dir, "worker.yaml"), filepath.Join(dir, "worker.json")
	require.NoError(t, os.WriteFile(yaml, []byte("queue: tls://file\nserver: https://file\nserver_key: файл\naccess_seed: зерно\nregion: eu-north-1\napp_key: приложение\n"), 0o600), "YAML не записан")
	require.NoError(t, os.WriteFile(json, []byte(`{"queue": "tls://json", "server": "https://json", "server_key": "json", "access_seed": "зерно-json", "region": "eu-west-1", "app_key": "приложение-json"}`), 0o600), "JSON не записан")
	client := filepath.Join(dir, "gophkeeper", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(client), 0o700), "каталог клиента не создан")
	require.NoError(t, os.WriteFile(client, []byte("server_key: клиент\n"), 0o600), "файл клиента не записан")
	t.Setenv("XDG_CONFIG_HOME", dir)
	queue := map[string]string{"GOPHKEEPER_QUEUE": "tls://env"}
	snaps.MatchSnapshot(t, []string{
		"file: " + loadWith(t, nil, "-c", yaml),
		"flag: " + loadWith(t, nil, "-c", yaml, "--queue", "tls://flag"),
		"env: " + loadWith(t, queue, "-c", yaml, "--queue", "tls://flag"),
		"json: " + loadWith(t, nil, "--config", json),
		"CONFIG: " + loadWith(t, map[string]string{"CONFIG": json}, "-c", yaml),
		"no file: " + loadWith(t, map[string]string{"GOPHKEEPER_QUEUE": "tls://env", "GOPHKEEPER_SERVER": "https://env"}),
		"broken: " + strings.ReplaceAll(loadWith(t, map[string]string{"CONFIG": dir}), dir, "<dir>"),
	})
}
