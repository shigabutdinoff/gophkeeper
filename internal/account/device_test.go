package account

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func (g *gotrue) registered() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.devices...)
}

func publicKey(sign ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(sign.Public().(ed25519.PublicKey))
}

func TestLoginRegistersDevice(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	r, ok := recall()
	require.True(t, ok, "записи в хранилище нет")
	require.Len(t, r.Sign, ed25519.SeedSize, "ключа устройства нет в хранилище")
	sign := ed25519.NewKeyFromSeed(r.Sign)
	assert.Equal(t, []string{"Bearer доступ " + publicKey(sign)}, g.registered(), "на сервере не тот ключ устройства")
}

func TestLoginDeviceRefused(t *testing.T) {
	g := startGotrue(t, false)
	g.refuse = true
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.Error(t, err, "вход без ключа устройства прошёл")
	_, ok := recall()
	assert.False(t, ok, "запись в хранилище осталась")
	_, err = loadEmail(g.settings.Server)
	assert.ErrorIs(t, err, ErrLoggedOut, "вход сохранён без ключа устройства")
}

func TestUnlockWithoutServer(t *testing.T) {
	cases := map[string]func(g *gotrue){
		"offline": func(g *gotrue) { g.srv.Close() },
		"server error": func(g *gotrue) {
			g.mu.Lock()
			g.fail = true
			g.mu.Unlock()
		},
	}
	for name, breakServer := range cases {
		t.Run(name, func(t *testing.T) {
			g := startGotrue(t, false)
			_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
			require.NoError(t, err, "вход не прошёл")
			breakServer(g)
			keys, err := Unlock(t.Context(), g.client, g.settings, noAsk)
			require.NoError(t, err, "ключи не получены без сервера")
			r, ok := recall()
			require.True(t, ok, "вход забыт без сервера")
			assert.Equal(t, r.Data, keys.Data, "ключ к данным не из хранилища")
			assert.Equal(t, ed25519.NewKeyFromSeed(r.Sign), keys.Sign, "ключ устройства не из хранилища")
		})
	}
}

func TestUnlockWithoutSignKey(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	r, _ := recall()
	r.Sign = nil
	require.NoError(t, keep(r), "запись не сохранена")
	_, err = Unlock(t.Context(), g.client, g.settings, noAsk)
	assert.ErrorIs(t, err, ErrLoggedOut, "вход без ключа устройства принят")
}

func TestUnlockWithoutKeyringOffline(t *testing.T) {
	g := startGotrue(t, false)
	keyring.MockInitWithError(errors.New("нет хранилища"))
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	g.srv.Close()
	_, err = Unlock(t.Context(), g.client, g.settings, func(string) (string, error) { return "пароль-Анны", nil })
	assert.ErrorIs(t, err, ErrNoAnswer, "без хранилища ключи выданы без сервера")
}
