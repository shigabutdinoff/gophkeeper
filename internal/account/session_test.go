package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

type gotrue struct {
	mu       sync.Mutex
	calls    []string
	refresh  int
	revoked  bool
	bodies   []map[string]string
	bearer   string
	settings config.Settings
	client   *http.Client
	srv      *httptest.Server
	devices  []string
	refuse   bool
	fail     bool
}

func startGotrue(t *testing.T, fail bool) *gotrue {
	t.Helper()
	g := &gotrue{fail: fail}
	wrong, _, err := Keys("anna@example.com", "неверный")
	require.NoError(t, err, "ключ не выведен")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.calls = append(g.calls, r.URL.Path+"?"+r.URL.RawQuery)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		g.bodies = append(g.bodies, body)
		switch {
		case g.fail:
			w.WriteHeader(http.StatusBadGateway)
		case r.Header.Get("apikey") != "ключ":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/rest/v1/device_keys" && g.refuse:
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"code":"42501"}`))
		case r.URL.Path == "/rest/v1/device_keys":
			g.devices = append(g.devices, r.Header.Get("Authorization")+" "+body["public_key"])
			w.WriteHeader(http.StatusCreated)
		case g.revoked && body["refresh_token"] != "":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"error_code":"refresh_token_not_found"}`))
		case r.URL.Path == "/auth/v1/logout":
			g.bearer = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusNoContent)
		case body["password"] == wrong:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"error_code":"invalid_credentials","msg":"Invalid login credentials"}`))
		default:
			g.refresh++
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "доступ", "refresh_token": "обновление-" + string(rune('0'+g.refresh))})
		}
	}))
	t.Cleanup(srv.Close)
	g.srv = srv
	g.client = srv.Client()
	g.client.Timeout = 2 * time.Second
	g.settings = config.Settings{Server: srv.URL, AppKey: "ключ"}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	keyring.MockInit()
	return g
}

func (g *gotrue) log(t *testing.T) string {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.calls, "\n")
}

func (g *gotrue) state() ([]map[string]string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]map[string]string(nil), g.bodies...), g.bearer
}

func (g *gotrue) revoke() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revoked = true
}

func noAsk(string) (string, error) { return "", errors.New("пароль не нужен") }

func TestLoginStoresKeyOutsideConfig(t *testing.T) {
	g := startGotrue(t, false)
	saved, err := Login(t.Context(), g.client, g.settings, " Anna@Example.COM ", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	assert.True(t, saved, "вход не сохранён в хранилище")
	auth, data, err := Keys("anna@example.com", "пароль-Анны")
	require.NoError(t, err, "ключи не выведены")
	bodies, _ := g.state()
	assert.Equal(t, map[string]string{"email": "anna@example.com", "password": auth}, bodies[0], "на сервер ушёл не ключ входа")
	r, ok := recall()
	require.True(t, ok, "записи в хранилище нет")
	assert.Equal(t, record{Server: g.settings.Server, Email: "anna@example.com", Data: data, Refresh: "обновление-1", AppKey: "ключ", Sign: r.Sign}, r, "в хранилище не тот ключ")
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	files, err := os.ReadDir(dir)
	require.NoError(t, err, "каталог настроек не прочитан")
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(dir, f.Name()))
		require.NoError(t, err, "файл не прочитан")
		assert.NotContains(t, string(content), data, "ключ к данным на диске")
		assert.NotContains(t, string(content), "обновление", "токен обновления на диске")
	}
	snaps.MatchSnapshot(t, g.log(t))
}

func TestLoginWithoutKeyring(t *testing.T) {
	g := startGotrue(t, false)
	keyring.MockInitWithError(errors.New("нет хранилища"))
	saved, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	assert.False(t, saved, "вход сохранён без хранилища")
	email, err := loadEmail(g.settings.Server)
	require.NoError(t, err, "email не сохранён")
	assert.Equal(t, "anna@example.com", email, "сохранён не тот email")
	asked := ""
	keys, err := Unlock(t.Context(), g.client, g.settings, func(e string) (string, error) { asked = e; return "пароль-Анны", nil })
	require.NoError(t, err, "ключ не получен по паролю")
	_, want, _ := Keys("anna@example.com", "пароль-Анны")
	assert.Equal(t, want, keys.Data, "ключ к данным не тот")
	assert.Equal(t, []string{"Bearer доступ " + publicKey(keys.Sign)}, g.registered()[1:], "одноразовый ключ устройства не зарегистрирован")
	assert.Equal(t, "anna@example.com", asked, "пароль спрошен не для email входа")
	_, err = Unlock(t.Context(), g.client, g.settings, noAsk)
	require.Error(t, err, "отказ ввода пароля не вернул ошибку")
	require.NoError(t, Logout(t.Context(), g.client), "выход без хранилища не прошёл")
}

func TestLoginErrors(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "неверный")
	assert.ErrorIs(t, err, ErrCredentials, "неверный пароль не распознан")
	assert.NotErrorIs(t, err, ErrServer, "неверный пароль принят за сбой")
	_, err = loadEmail(g.settings.Server)
	assert.ErrorIs(t, err, ErrLoggedOut, "вход сохранён после ошибки")
	g = startGotrue(t, true)
	_, err = Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	assert.ErrorIs(t, err, ErrServer, "сбой сервера не распознан")
}

func TestUnlockRotatesRefresh(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Unlock(t.Context(), g.client, g.settings, noAsk)
	require.ErrorIs(t, err, ErrLoggedOut, "без входа ключ выдан")
	assert.ErrorContains(t, err, "сначала войдите в GophKeeper", "ошибка без входа не просит войти")
	_, err = Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	for range 2 {
		_, err = Unlock(t.Context(), g.client, g.settings, noAsk)
		require.NoError(t, err, "ключ не получен по токену обновления")
	}
	r, _ := recall()
	assert.Equal(t, "обновление-3", r.Refresh, "новый токен обновления не сохранён")
	snaps.MatchSnapshot(t, g.log(t))
	bodies, _ := g.state()
	snaps.MatchJSON(t, bodies[2:])
}

func TestLogout(t *testing.T) {
	g := startGotrue(t, false)
	require.NoError(t, Logout(t.Context(), g.client), "выход без входа вернул ошибку")
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	require.NoError(t, Logout(t.Context(), g.client), "выход не прошёл")
	_, bearer := g.state()
	assert.Equal(t, "Bearer доступ", bearer, "выход без токена доступа")
	snaps.MatchSnapshot(t, g.log(t))
	assertForgotten(t, g)
}

func TestLogoutWithoutServer(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	require.NoError(t, keep(record{Server: "https://127.0.0.1:1", Data: "д", Refresh: "о"}), "запись не сохранена")
	require.NoError(t, Logout(t.Context(), g.client), "выход без сервера не прошёл")
	assertForgotten(t, g)
}

func assertForgotten(t *testing.T, g *gotrue) {
	t.Helper()
	_, ok := recall()
	assert.False(t, ok, "запись в хранилище осталась")
	_, err := loadEmail(g.settings.Server)
	assert.ErrorIs(t, err, ErrLoggedOut, "файл входа остался")
}

func TestUnlockIgnoresOtherAccount(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	path, err := sessionPath()
	require.NoError(t, err, "путь входа не найден")
	require.NoError(t, os.WriteFile(path, []byte("server: "+g.settings.Server+"\nemail: boris@example.com\n"), 0o600), "файл входа не записан")
	asked := ""
	keys, err := Unlock(t.Context(), g.client, g.settings, func(e string) (string, error) { asked = e; return "пароль-Бориса", nil })
	require.NoError(t, err, "ключ не получен по паролю")
	_, want, _ := Keys("boris@example.com", "пароль-Бориса")
	assert.Equal(t, want, keys.Data, "выдан ключ чужой записи")
	assert.Equal(t, "boris@example.com", asked, "пароль спрошен не для email входа")
}

func TestLogoutAfterServerChange(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	other := config.Settings{Server: "https://127.0.0.1:1", AppKey: "другой"}
	_, err = Unlock(t.Context(), g.client, other, noAsk)
	require.ErrorIs(t, err, ErrLoggedOut, "вход другого сервера принят")
	require.NoError(t, Logout(t.Context(), g.client), "выход не прошёл")
	_, bearer := g.state()
	assert.Equal(t, "Bearer доступ", bearer, "сессия сервера входа не завершена")
	assertForgotten(t, g)
}

func TestUnlockRevokedSession(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	g.revoke()
	_, err = Unlock(t.Context(), g.client, g.settings, noAsk)
	require.ErrorIs(t, err, ErrLoggedOut, "отозванная сессия не считается выходом")
	assertForgotten(t, g)
}

func TestLoginConfigDirIsFile(t *testing.T) {
	g := startGotrue(t, false)
	dir, err := config.Dir()
	require.NoError(t, err, "каталог настроек не найден")
	require.NoError(t, os.MkdirAll(filepath.Dir(dir), 0o700), "родитель не создан")
	require.NoError(t, os.WriteFile(dir, nil, 0o600), "файл не записан")
	_, err = Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.Error(t, err, "вход без файла входа сообщил успех")
}

func TestLogoutLockedKeyring(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	keyring.MockInitWithError(errors.New("заблокировано"))
	require.Error(t, Logout(t.Context(), g.client), "выход при запертом хранилище прошёл")
	path, err := sessionPath()
	require.NoError(t, err, "путь входа не найден")
	assert.FileExists(t, path, "файл входа удалён")
}

func TestLogoutBrokenSessionLockedKeyring(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	path, err := sessionPath()
	require.NoError(t, err, "путь входа не найден")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600), "файл входа не записан")
	keyring.MockInitWithError(errors.New("заблокировано"))
	require.Error(t, Logout(t.Context(), g.client), "выход при запертом хранилище прошёл")
	assert.FileExists(t, path, "файл входа удалён")
}
