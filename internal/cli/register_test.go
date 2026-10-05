package cli

import (
	"cmp"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const (
	typedEmail    = " Anna@Example.COM "
	typedPassword = "пароль-Анны"
)

type signupServer struct {
	*httptest.Server
	requests atomic.Int32
	body     map[string]string
	apikey   string
	paths    []string
}

func startServer(t *testing.T, reply http.HandlerFunc) *signupServer {
	t.Helper()
	s := &signupServer{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		s.paths = append(s.paths, r.URL.Path)
		if r.URL.Path == "/auth/v1/signup" {
			s.apikey = r.Header.Get("apikey")
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&s.body), "тело запроса не разобрано")
		}
		reply(w, r)
	}))
	t.Cleanup(s.Close)
	client := s.Client()
	client.Timeout = 200 * time.Millisecond
	setVar(t, &httpClient, client)
	return s
}

func setVar[T any](t *testing.T, v *T, value T) {
	t.Helper()
	old := *v
	*v = value
	t.Cleanup(func() { *v = old })
}

func keyboard(t *testing.T, terminal bool, password string) *os.File {
	t.Helper()
	setVar(t, &isTerminal, func(int) bool { return terminal })
	setVar(t, &readPassword, func(int) ([]byte, error) { return []byte(password), nil })
	path := filepath.Join(t.TempDir(), "stdin")
	require.NoError(t, os.WriteFile(path, []byte(typedEmail+"\n"), 0o600), "email не записан во ввод")
	in, err := os.Open(path)
	require.NoError(t, err, "файл ввода не открыт")
	t.Cleanup(func() { _ = in.Close() })
	return in
}

func settingsFile(t *testing.T, server, appKey string) string {
	t.Helper()
	return writeSettings(t, config.Settings{Server: server, AppKey: appKey})
}

func writeSettings(t *testing.T, s config.Settings) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv(serverEnv, "")
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	require.NoError(t, os.MkdirAll(dir, 0o700), "каталог настроек не создан")
	data, err := yaml.Marshal(s)
	require.NoError(t, err, "настройки не размечены")
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.File), data, 0o600), "файл настроек не записан")
	return home
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte(`{"access_token":"доступ","refresh_token":"обновление"}`))
}

func TestRegisterHidesPassword(t *testing.T) {
	srv := startServer(t, ok)
	settingsFile(t, srv.URL, "ключ-приложения")
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "register")
	require.NoError(t, err, "регистрация не прошла")
	snaps.MatchSnapshot(t, out, errOut)
	snaps.MatchJSON(t, srv.body)
	assert.Equal(t, []string{"/auth/v1/signup", "/auth/v1/token", "/rest/v1/device_keys"}, srv.paths, "после регистрации вход не выполнен или ключ устройства не зарегистрирован")
	assert.Equal(t, "ключ-приложения", srv.apikey, "ключ приложения не из файла настроек")
	for _, v := range srv.body {
		assert.NotContains(t, v, typedPassword, "пароль ушёл на сервер")
	}
}

func TestRegisterHidesServerDetails(t *testing.T) {
	srv := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error_code":"secret-detail","msg":"secret-detail"}`))
	})
	settingsFile(t, srv.URL, "ключ-приложения")
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "register")
	require.ErrorIs(t, err, account.ErrServer, "регистрация прошла при ошибке сервера")
	assert.Empty(t, out, "при ошибке напечатан stdout")
	assert.NotContains(t, err.Error()+errOut, "secret-detail", "ответ сервера попал на экран")
}

func TestRegisterRefusesBeforeSending(t *testing.T) {
	cases := map[string]struct {
		server, flag, password string
		noKey, notTerminal     bool
	}{
		"http":           {server: "http://example.invalid"},
		"no server":      {},
		"no key":         {server: "$SERVER", noKey: true},
		"foreign server": {server: "$SERVER", flag: "https://other.invalid"},
		"short":          {server: "$SERVER", password: "1234567"},
		"not terminal":   {server: "$SERVER", notTerminal: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := startServer(t, ok)
			appKey := "к"
			if c.noKey {
				appKey = ""
			}
			home := settingsFile(t, strings.ReplaceAll(c.server, "$SERVER", srv.URL), appKey)
			in := keyboard(t, !c.notTerminal, cmp.Or(c.password, typedPassword))
			_, _, err := executeIn(in, "register", "--server="+c.flag)
			require.Error(t, err, "регистрация прошла")
			assert.Zero(t, srv.requests.Load(), "клиент отправил запрос на сервер")
			snaps.MatchSnapshot(t, strings.NewReplacer(home, "$XDG_CONFIG_HOME", srv.URL, "$SERVER").Replace(err.Error()))
		})
	}
}

func TestRegisterServerPriority(t *testing.T) {
	for _, c := range []struct{ name, env, flag, want string }{
		{"env", "https://env.invalid", "https://flag.invalid", "https://env.invalid"},
		{"flag", "", "https://flag.invalid", "https://flag.invalid"},
		{"file", "", "", "https://file.invalid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			settingsFile(t, "https://file.invalid", "")
			t.Setenv(serverEnv, c.env)
			_, err := resolve(c.flag)
			assert.ErrorContains(t, err, "для сервера "+c.want+":", "выбран не адрес с высшим приоритетом")
		})
	}
}
