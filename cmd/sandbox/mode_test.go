package main

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func ownState(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	require.NoError(t, ownKeys(state, true), "ключи проекта не созданы")
	require.NoError(t, sandboxSecrets(state), "секреты песочницы не созданы")
	return state
}

func TestBackend(t *testing.T) {
	t.Setenv(supabaseURL, "https://project.supabase.co")
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "ключ администратора")
	c, cert, err := backend(t.TempDir())
	require.NoError(t, err, "облачный режим не прочитан из окружения")
	assert.Equal(t, []string{"https://project.supabase.co", "ключ", "ключ администратора", ""},
		[]string{string(c.Supabase.URL), c.Supabase.Key, c.ServiceKey, cert}, "облачный режим взят не из окружения")

	for _, name := range []string{supabaseURL, supabaseKey, "SUPABASE_REGION", "SUPABASE_SERVICE_ROLE_KEY"} {
		t.Setenv(name, "")
	}
	state := ownState(t)
	app, err := os.ReadFile(filepath.Join(state, appKeyFile))
	require.NoError(t, err, "ключ приложения не создан")
	service, err := os.ReadFile(filepath.Join(state, serviceKeyFile))
	require.NoError(t, err, "служебный ключ не создан")
	assert.NotEqual(t, app, service, "ключи проекта совпали")
	c, cert, err = backend(state)
	require.NoError(t, err, "режим своего сервера не прочитан из каталога состояния")
	assert.Equal(t, []string{"https://" + ownAddr, string(app), string(service), filepath.Join(state, caFile)},
		[]string{string(c.Supabase.URL), c.Supabase.Key, c.ServiceKey, cert}, "режим своего сервера взят не из каталога состояния")
	require.NoError(t, os.Remove(filepath.Join(state, appKeyFile)), "ключ приложения не удалён")
	_, _, err = backend(state)
	assert.Error(t, err, "режим своего сервера прочитан без ключа приложения")
}

func TestCheckMode(t *testing.T) {
	const refusal = "песочница создана в другом режиме, сначала выполните go run ./cmd/sandbox reset"
	own, cloud, fresh := ownState(t), t.TempDir(), t.TempDir()
	for _, name := range []string{supabaseURL, supabaseKey, "SUPABASE_REGION", "SUPABASE_SERVICE_ROLE_KEY"} {
		t.Setenv(name, "")
	}
	assert.EqualError(t, checkMode(fresh, false), "не заданы SUPABASE_URL, SUPABASE_ANON_KEY, SUPABASE_REGION, SUPABASE_SERVICE_ROLE_KEY")
	assert.NoError(t, checkMode(fresh, true), "пустой каталог не принят режимом своего сервера")
	assert.NoError(t, checkMode(own, true), "каталог своего режима не принят")
	assert.EqualError(t, checkMode(own, false), refusal)
	t.Setenv(supabaseURL, "https://project.supabase.co")
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "ключ администратора")
	require.NoError(t, checkMode(cloud, false), "пустой каталог не принят облачным режимом")
	require.NoError(t, ownKeys(cloud, false), "облачный режим отказал в подготовке")
	require.NoError(t, sandboxSecrets(cloud), "секреты песочницы не созданы")
	assert.NoFileExists(t, filepath.Join(cloud, serviceKeyFile), "в облачном каталоге появился служебный ключ")
	assert.NoError(t, checkMode(cloud, false), "каталог облачного режима не принят")
	assert.EqualError(t, checkMode(cloud, true), refusal)
	before := readFiles(t, own)
	require.NoError(t, ownKeys(own, true), "повторная подготовка своего режима не прошла")
	assert.Equal(t, before, readFiles(t, own), "повторная подготовка перевыпустила секреты")
}

func TestOwnKeysExistingState(t *testing.T) {
	state := ownState(t)
	info, err := os.Stat(filepath.Join(state, tokenKeyFile))
	require.NoError(t, err, "ключ подписи токенов не создан")
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "ключ подписи токенов читают другие пользователи")
	}
	require.NoError(t, os.Remove(filepath.Join(state, tokenKeyFile)), "ключ подписи токенов не удалён")
	before := readFiles(t, state)
	require.NoError(t, ownKeys(state, true), "готовый каталог состояния не дополнен")
	after := readFiles(t, state)
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(after[tokenKeyFile]))
	require.NoError(t, err, "ключ подписи токенов не читается сервером")
	assert.Equal(t, "P-256", key.Curve.Params().Name, "ключ подписи токенов не на кривой P-256")
	block, _ := pem.Decode([]byte(after[tokenKeyFile]))
	require.NotNil(t, block, "ключ подписи токенов не PEM")
	assert.Equal(t, "PRIVATE KEY", block.Type, "ключ подписи токенов не в PKCS8")
	delete(after, tokenKeyFile)
	assert.Equal(t, before, after, "дополнение каталога перевыпустило прежние секреты")
	cloud := t.TempDir()
	require.NoError(t, ownKeys(cloud, false), "облачный режим отказал в подготовке")
	assert.NoFileExists(t, filepath.Join(cloud, tokenKeyFile), "в облачном каталоге появился ключ подписи токенов")
}

func TestOwnServer(t *testing.T) {
	if os.Getenv(testDBEnv) == "" {
		t.Skip("не задана " + testDBEnv)
	}
	state := ownState(t)
	own, err := ownServer(state)
	require.NoError(t, err, "адрес своего сервера не открыт")
	_, err = ownServer(state)
	assert.ErrorContains(t, err, "адрес своего сервера "+ownAddr+" не открыт: ", "занятый адрес открыт повторно")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- own(ctx) }()
	c, cert, err := backend(state)
	require.NoError(t, err, "режим своего сервера не прочитан")
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, string(c.Supabase.URL)+"/rest/v1/records", nil)
	require.NoError(t, err, "запрос не собран")
	req.Header.Set("apikey", c.ServiceKey)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	require.Error(t, err, "клиент без доверия центру песочницы получил ответ")
	require.NoError(t, account.Trust(client, cert), "центр песочницы не принят")
	resp, err = client.Do(req)
	require.NoError(t, err, "свой сервер не ответил обработчику")
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "служебный ключ не принят")
	resp, err = client.Get(string(c.Supabase.URL) + "/auth/v1/.well-known/jwks.json")
	require.NoError(t, err, "свой сервер не отдал ключи")
	keys, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err, "ключи не прочитаны")
	assert.Contains(t, string(keys), `"alg":"ES256"`, "ключ подписи токенов песочницы не принят сервером")
	cancel()
	require.NoError(t, <-done, "свой сервер остановился с ошибкой")

	t.Setenv(supabaseURL, "https://project.supabase.co")
	t.Setenv(supabaseKey, "ключ")
	t.Setenv("SUPABASE_REGION", "eu-north-1")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "ключ администратора")
	none, err := ownServer(t.TempDir())
	require.NoError(t, err, "облачный режим открыл адрес своего сервера")
	assert.NoError(t, none(t.Context()), "облачный режим запустил свой сервер")
}
