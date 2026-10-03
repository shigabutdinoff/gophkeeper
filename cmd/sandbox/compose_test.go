package main

import (
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type composeService struct {
	Image       string            `yaml:"image"`
	Restart     string            `yaml:"restart"`
	PullPolicy  string            `yaml:"pull_policy"`
	Command     []string          `yaml:"command"`
	Entrypoint  []string          `yaml:"entrypoint"`
	Environment map[string]string `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	PreStop     []struct {
		Command []string `yaml:"command"`
	} `yaml:"pre_stop"`
}

type composeFile struct {
	Name     string                    `yaml:"name"`
	Services map[string]composeService `yaml:"services"`
	Configs  map[string]struct {
		Content string `yaml:"content"`
	} `yaml:"configs"`
}

func parseModel(t *testing.T) composeFile {
	data, err := os.ReadFile(filepath.Join("..", "..", "compose.yaml"))
	require.NoError(t, err, "модель песочницы не прочитана")
	var f composeFile
	require.NoError(t, yaml.Unmarshal(data, &f), "модель песочницы не разбирается")
	return f
}

func TestModelPorts(t *testing.T) {
	var published []string
	for name, s := range parseModel(t).Services {
		for _, p := range s.Ports {
			assert.True(t, strings.HasPrefix(p, "127.0.0.1:"), "порт %q службы %q открыт не только на этой машине", p, name)
		}
		published = append(published, s.Ports...)
	}
	assert.Equal(t, []string{"127.0.0.1:54222:4222"}, published, "опубликованы не те порты")
}

func TestModelSecretsReadOnly(t *testing.T) {
	for name, s := range parseModel(t).Services {
		for _, v := range s.Volumes {
			if strings.HasPrefix(v, "sandbox:") && name != "init" {
				assert.True(t, strings.HasSuffix(v, ":ro"), "служба %q может менять секреты песочницы", name)
			}
		}
	}
}

func TestModelPolicies(t *testing.T) {
	f := parseModel(t)
	assert.Equal(t, "gophkeeper-sandbox", f.Name, "проект Compose назван иначе")
	for name, s := range f.Services {
		assert.Equal(t, "missing", s.PullPolicy, "служба %q скачивает образ при каждом запуске", name)
		want := "unless-stopped"
		if name == "init" {
			want = "no"
		}
		assert.Equal(t, want, s.Restart, "у службы %q не та политика перезапуска", name)
	}
}

func TestModelServices(t *testing.T) {
	f := parseModel(t)
	for _, name := range slices.Sorted(maps.Keys(f.Services)) {
		s := f.Services[name]
		snaps.MatchSnapshot(t, name, s.Image, s.Entrypoint, s.Command, s.Environment, s.Volumes)
	}
	snaps.MatchSnapshot(t, f.Configs["nats"].Content)
}

func TestModelQueueStrings(t *testing.T) {
	cfg, err := loadStream()
	require.NoError(t, err, "поток изменений не прочитан")
	f := parseModel(t)
	conf := f.Configs["nats"].Content
	assert.Contains(t, conf, `publish: {allow: ["`+cfg.Subjects[0]+`"]}`, "клиент публикует не в поток изменений")
	assert.Contains(t, conf, `subscribe: {allow: ["`+queueInbox+`.>"]}`, "клиент слушает не свои ответы")
	assert.Contains(t, f.Services["nats"].Ports, strings.TrimPrefix(sandboxQueue, "tls://")+":4222", "песочница пишет клиенту не тот адрес очереди")
	command := f.Services["init"].Command
	require.NotEmpty(t, command, "служба init ничего не запускает")
	cmd, _, err := newRoot(io.Discard, io.Discard).Find(command[len(command)-1:])
	require.NoError(t, err, "служба init запускает неизвестную команду")
	assert.Equal(t, "init", cmd.Name(), "служба init запускает не ту команду")
}

func TestModelStopHook(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("нет sh для проверки хука остановки")
	}
	f := parseModel(t)
	i := slices.IndexFunc(f.Services["init"].Volumes, func(v string) bool { return strings.HasSuffix(v, ":/config") })
	require.NotEqual(t, -1, i, "init не видит каталог настроек")
	assert.Contains(t, f.Services["settings"].Volumes, f.Services["init"].Volumes[i], "хук видит не тот каталог настроек, что init")
	hooks := f.Services["settings"].PreStop
	require.Len(t, hooks, 1, "у settings не один хук остановки")
	require.Len(t, hooks[0].Command, 3, "хук остановки запускается не через sh -c")
	aside := filepath.Base(asidePath(settingsFile, "sandbox"))
	sandbox, cloud := marked("sandbox", []byte("песочница\n")), marked("cloud", []byte("облако\n"))
	for _, files := range []map[string][]byte{
		{},
		{settingsFile: sandbox},
		{settingsFile: []byte("личное\n"), aside: sandbox},
		{settingsFile: cloud, aside: []byte("личное\n")},
	} {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "gophkeeper"), 0o700), "каталог настроек не создан")
		for name, data := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gophkeeper", name), data, 0o600), "файл %q не записан", name)
		}
		script := strings.NewReplacer("$$", "$", "/config/", dir+"/").Replace(hooks[0].Command[2])
		out, err := exec.Command(sh, "-c", script).CombinedOutput()
		require.NoError(t, err, "хук остановки упал: %s", out)
		entries, err := os.ReadDir(filepath.Join(dir, "gophkeeper"))
		require.NoError(t, err, "каталог настроек не прочитан")
		var left []string
		for _, e := range entries {
			left = append(left, e.Name())
		}
		snaps.MatchSnapshot(t, left)
	}
}

func TestModelGoImage(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.GoVersion}}").Output()
	require.NoError(t, err, "версия Go из go.mod не получена")
	want := "golang:" + strings.TrimSpace(string(out)) + "-alpine"
	assert.Equal(t, want, parseModel(t).Services["init"].Image, "образ init расходится с версией Go в go.mod")
}
