package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cloudVars = []string{supabaseURL, supabaseKey, "SUPABASE_REGION", adminCreds, clientCreds}

func clearCloudEnv(t *testing.T) {
	for _, k := range cloudVars {
		t.Setenv(k, "")
	}
}

func startCloudQueue(t *testing.T, users ...*server.User) string {
	opts := natstest.DefaultTestOptions
	opts.Port, opts.JetStream, opts.StoreDir = server.RANDOM_PORT, true, t.TempDir()
	opts.Users = append([]*server.User{{Username: "admin", Password: "admin-pass"}}, users...)
	s := natstest.RunServer(&opts)
	t.Cleanup(s.Shutdown)
	require.NoError(t, s.GlobalAccount().UpdateJetStreamLimits(map[string]server.JetStreamAccountLimits{
		"": {MaxMemory: -1, MaxStore: -1, MaxStreams: -1, MaxConsumers: -1, MaxAckPending: -1,
			MemoryMaxStreamBytes: -1, StoreMaxStreamBytes: -1, MaxBytesRequired: true},
	}), "очередь не требует предела размера потока, как Synadia")
	return s.ClientURL()
}

func cloudValues(t *testing.T) cloudConfig {
	creds := filepath.Join(t.TempDir(), "client.creds")
	require.NoError(t, os.WriteFile(creds, []byte("ЗАГЛУШКА CREDS\n"), 0o600), "учётные данные не записаны")
	return cloudConfig{Supabase: supabaseConfig{URL: "https://project.supabase.co", Key: "ЗАГЛУШКА КЛЮЧА", Region: "eu-north-1"}, ClientCreds: creds}
}

func TestConnectCloud(t *testing.T) {
	url := startCloudQueue(t)
	admin := nats.UserInfo("admin", "admin-pass")
	q := queueAccess{url, admin, nats.UserInfo("client", "client-pass")}
	dir := t.TempDir()
	state, clientPath := filepath.Join(dir, "cloud"), filepath.Join(dir, "config.yaml")
	v := cloudValues(t)
	msg, err := connectCloud(t.Context(), v, sandboxPaths{state, clientPath}, q)
	require.NoError(t, err, "облако не подготовлено")
	nc, err := nats.Connect(url, admin)
	require.NoError(t, err, "администратор не подключился")
	defer nc.Close()
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	_, err = js.Publish(context.Background(), "changes.anna", []byte("изменение Анны"))
	require.NoError(t, err, "изменение не принято")
	again, err := connectCloud(t.Context(), v, sandboxPaths{state, clientPath}, q)
	require.NoError(t, err, "повторная подготовка не прошла")
	info, err := js.Stream(context.Background(), "CHANGES")
	require.NoError(t, err, "поток не найден")
	assert.Equal(t, uint64(1), info.CachedInfo().State.Msgs, "повторная подготовка потеряла изменения")
	settings, err := os.ReadFile(clientPath)
	require.NoError(t, err, "файл настроек не записан")
	creds, err := os.ReadFile(filepath.Join(state, credsFile))
	require.NoError(t, err, "учётные данные не скопированы")
	assert.Equal(t, "ЗАГЛУШКА CREDS\n", string(creds), "скопированы не те учётные данные")
	snaps.MatchSnapshot(t, strings.NewReplacer(dir, "<dir>", url, "<queue>").Replace(strings.Join([]string{msg.String(), again.String(), string(settings)}, "\n")))
}

func TestConnectCloudOpenQueue(t *testing.T) {
	url := startCloudQueue(t, &server.User{Username: "client", Password: "client-pass"})
	dir := t.TempDir()
	_, err := connectCloud(t.Context(), cloudValues(t), sandboxPaths{filepath.Join(dir, "cloud"), filepath.Join(dir, "config.yaml")},
		queueAccess{url, nats.UserInfo("admin", "admin-pass"), nats.UserInfo("client", "client-pass")})
	require.Error(t, err, "очередь, которая пускает устройство без входа, принята")
	assert.NoDirExists(t, filepath.Join(dir, "cloud"), "настройки записаны при отказе")
	snaps.MatchSnapshot(t, err.Error())
}

func TestConnectCloudNoCreds(t *testing.T) {
	v := cloudValues(t)
	v.ClientCreds = filepath.Join(t.TempDir(), "нет.creds")
	_, err := connectCloud(t.Context(), v, sandboxPaths{t.TempDir(), filepath.Join(t.TempDir(), "config.yaml")}, queueAccess{URL: "nats://127.0.0.1:1"})
	require.Error(t, err, "отсутствующие учётные данные приняты")
	assert.Contains(t, err.Error(), "учётные данные клиента очереди", "ошибка не называет учётные данные")
}

func TestCloudEnv(t *testing.T) {
	clearCloudEnv(t)
	_, err := parseEnv[cloudConfig]()
	require.Error(t, err, "пустое окружение принято")
	t.Setenv(supabaseURL, "http://project.supabase.co")
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv(adminCreds, "admin.creds")
	t.Setenv(clientCreds, "client.creds")
	_, plain := parseEnv[cloudConfig]()
	require.Error(t, plain, "адрес без HTTPS принят")
	t.Setenv(supabaseURL, "https://project.supabase.co")
	v, ok := parseEnv[cloudConfig]()
	require.NoError(t, ok, "полное окружение отклонено")
	assert.Equal(t, "client.creds", v.ClientCreds, "окружение прочитано не так")
	snaps.MatchSnapshot(t, err.Error(), plain.Error())
}

func TestCloudCommand(t *testing.T) {
	clearCloudEnv(t)
	var out, errOut strings.Builder
	root := newRoot(&out, &errOut)
	root.SetArgs([]string{"cloud"})
	require.Error(t, root.Execute(), "команда без окружения выполнилась")
	assert.Empty(t, out.String(), "команда без окружения что-то вывела")
	p, err := paths("cloud")
	require.NoError(t, err, "пути настроек не получены")
	assert.Equal(t, filepath.Join(filepath.Dir(p.client), "cloud"), p.state, "каталог облака не рядом с файлом настроек")
}
