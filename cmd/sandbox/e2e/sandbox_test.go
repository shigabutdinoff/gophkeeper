//go:build e2e

package e2e

import (
	"cmp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shigabutdinoff/gophkeeper/cmd/sandbox/internal/sandboxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type sandbox struct {
	root, settings string
	env            []string
}

func newSandbox(t *testing.T) *sandbox {
	for _, args := range [][]string{{"ps", "-aq"}, {"volume", "ls", "-q"}} {
		require.Empty(t, docker(t, append(args, "--filter", sandboxtest.Label)...), "в Docker уже есть песочница, прогон её не трогает")
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err, "домашний каталог не найден")
	tmp := t.TempDir()
	s := &sandbox{
		root:     filepath.Join("..", "..", ".."),
		settings: filepath.Join(sandboxtest.ConfigDir(tmp), "gophkeeper", "config.yaml"),
		env: append(os.Environ(), append(sandboxtest.HomeEnv(tmp), "SUPABASE_URL=https://project.supabase.co",
			"SUPABASE_ANON_KEY=e2e", "DOCKER_CONFIG="+cmp.Or(os.Getenv("DOCKER_CONFIG"), filepath.Join(home, ".docker")))...),
	}
	require.NoError(t, os.MkdirAll(sandboxtest.ConfigDir(tmp), 0o700), "каталог настроек не создан")
	t.Cleanup(func() { s.compose(t, "down", "-v") })
	return s
}

func (s *sandbox) run(args ...string) (string, error) {
	cmd := exec.Command("docker", append([]string{"compose", "-f", "compose.yaml", "-p", sandboxtest.Project}, args...)...)
	cmd.Dir, cmd.Env = s.root, s.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s *sandbox) compose(t *testing.T, args ...string) string {
	out, err := s.run(args...)
	require.NoError(t, err, "команда %q песочницы не выполнена: %s", args, out)
	return out
}

func (s *sandbox) start(t *testing.T) string {
	s.compose(t, "up", "-d", "--wait")
	return strings.TrimSpace(s.compose(t, "logs", "--no-log-prefix", "--tail", "1", "init"))
}

func (s *sandbox) client(t *testing.T) map[string]string {
	data, err := os.ReadFile(s.settings)
	require.NoError(t, err, "файл настроек не прочитан")
	var values map[string]string
	require.NoError(t, yaml.Unmarshal(data, &values), "файл настроек не разобран")
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, []byte(values["queue_ca"]), 0o600), "CA песочницы не записан")
	values["queue_ca_file"] = ca
	return values
}

func docker(t *testing.T, args ...string) []string {
	out, err := exec.Command("docker", args...).Output()
	require.NoError(t, err, "команда %q не выполнена", append([]string{"docker"}, args...))
	return strings.Fields(string(out))
}

func TestSandbox(t *testing.T) {
	s := newSandbox(t)
	busy := docker(t, "run", "-d", "--rm", "-p", "127.0.0.1:54222:4222", "nats:2.15.0-alpine")
	t.Cleanup(func() { _ = exec.Command("docker", append([]string{"rm", "-f"}, busy...)...).Run() })
	_, err := s.run("up", "-d", "--wait")
	assert.Error(t, err, "песочница поднялась на занятом порту очереди")
	assert.FileExists(t, s.settings, "init не записал файл настроек до отказа очереди")
	s.compose(t, "down")
	assert.NoFileExists(t, s.settings, "остановка после отказа очереди оставила файл настроек песочницы")
	docker(t, append([]string{"rm", "-f"}, busy...)...)

	personal := []byte("server: https://example.com\n")
	require.NoError(t, os.MkdirAll(filepath.Dir(s.settings), 0o700), "каталог настроек не создан")
	require.NoError(t, os.WriteFile(s.settings, personal, 0o600), "личный файл настроек не записан")
	assert.Contains(t, s.start(t), "другие настройки", "запуск не сообщил о личном файле")
	got, err := os.ReadFile(s.settings)
	require.NoError(t, err, "личный файл настроек не прочитан")
	assert.Equal(t, personal, got, "запуск изменил личный файл настроек")

	require.NoError(t, os.Remove(s.settings), "личный файл настроек не удалён")
	assert.Contains(t, s.start(t), "файл настроек клиента записан", "запуск не записал файл настроек")
	c := s.client(t)
	secrets := map[string]string{"admin": strings.TrimSpace(s.compose(t, "exec", "-T", "nats", "cat", "/etc/nats/sandbox/admin")),
		"client": c["queue_password"]}
	checkPorts(t)
	checkQueue(t, c)
	checkLogs(t, secrets)

	assert.Contains(t, s.start(t), "уже содержит эти настройки", "запуск не узнал файл песочницы")
	s.compose(t, "stop")
	assert.NoFileExists(t, s.settings, "остановка оставила файл настроек песочницы")
	assert.Contains(t, s.start(t), "файл настроек клиента записан", "запуск после остановки не записал файл песочницы")
	assert.Equal(t, "изменение Бориса", lastChange(t, c, secrets), "изменение пропало после перезапуска")
	assert.Equal(t, c["queue_ca"], s.client(t)["queue_ca"], "CA перевыпущен при перезапуске")

	s.compose(t, "down", "-v")
	assert.Empty(t, docker(t, "volume", "ls", "-q", "--filter", sandboxtest.Label), "сброс оставил тома песочницы")
	assert.NoFileExists(t, s.settings, "сброс оставил файл настроек песочницы")
	home := t.TempDir()
	s.env = append(s.env, sandboxtest.HomeEnv(home)...)
	s.settings = filepath.Join(sandboxtest.ConfigDir(home), "gophkeeper", "config.yaml")
	s.env = append(s.env, "SUPABASE_URL=")
	_, err = s.run("up", "-d", "--wait")
	assert.Error(t, err, "песочница поднялась без SUPABASE_URL")
	assert.NoError(t, os.Mkdir(filepath.Join(sandboxtest.ConfigDir(home), "other"), 0o700), "отказ init оставил каталог настроек за root")
	s.env = append(s.env, "SUPABASE_URL=https://project.supabase.co")
	assert.Contains(t, s.start(t), "файл настроек клиента записан", "запуск после сброса не записал файл песочницы")
	assert.NotEqual(t, c["queue_ca"], s.client(t)["queue_ca"], "после сброса CA прежний")
}

func checkLogs(t *testing.T, secrets map[string]string) {
	logs, err := exec.Command("docker", "compose", "-p", sandboxtest.Project, "logs", "--no-color").CombinedOutput()
	require.NoError(t, err, "журналы песочницы не прочитаны")
	for name, value := range secrets {
		assert.NotContains(t, string(logs), value, "секрет %q попал в журнал", name)
	}
}
