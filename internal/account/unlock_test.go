package account

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestUnlockWithoutServer(t *testing.T) {
	cases := map[string]func(g *gotrue){
		"offline":      func(g *gotrue) { g.srv.Close() },
		"server error": func(g *gotrue) { g.set(func(g *gotrue) { g.fail = true }) },
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
			assert.Equal(t, []string{"доступ", annaID}, []string{keys.Access, keys.User}, "токен доступа и пользователь не из хранилища")
			assert.Error(t, keys.Down, "нет причины сбоя")
		})
	}
}

func TestUnlockKeepsAccess(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	g.set(func(g *gotrue) { g.access = "доступ-2" })
	keys, err := Unlock(t.Context(), g.client, g.settings, noAsk)
	require.NoError(t, err, "ключи не получены")
	assert.Equal(t, "доступ-2", keys.Access, "выдан не новый токен доступа")
	assert.NoError(t, keys.Down, "причина сбоя при живом сервере")
	g.srv.Close()
	keys, err = Unlock(t.Context(), g.client, g.settings, noAsk)
	require.NoError(t, err, "ключи не получены без сервера")
	assert.Equal(t, []string{"доступ-2", annaID}, []string{keys.Access, keys.User}, "без сервера выдан не последний токен доступа")
	assert.ErrorIs(t, keys.Down, ErrNoAnswer, "нет причины сбоя")
	r, _ := recall()
	assert.Equal(t, "обновление-2", r.Refresh, "токен обновления потерян без сервера")
}

func TestUnlockOldRecord(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	r, _ := recall()
	r.Access, r.User = "", ""
	require.NoError(t, keep(r), "запись не сохранена")
	g.set(func(g *gotrue) { g.fail = true })
	keys, err := Unlock(t.Context(), g.client, g.settings, noAsk)
	require.NoError(t, err, "старый вход без сервера не принят")
	assert.Empty(t, keys.Access, "токен доступа взят из ниоткуда")
	assert.ErrorIs(t, keys.Down, ErrServer, "нет причины сбоя")
	g.set(func(g *gotrue) { g.fail = false })
	keys, err = Unlock(t.Context(), g.client, g.settings, noAsk)
	require.NoError(t, err, "старый вход не обновлён")
	assert.Equal(t, []string{"доступ", annaID}, []string{keys.Access, keys.User}, "старая запись входа не дополнена")
	r, _ = recall()
	assert.Equal(t, []string{"доступ", annaID}, []string{r.Access, r.User}, "токен доступа не сохранён")
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

func TestUnlockRetriesBusyRefresh(t *testing.T) {
	g := startGotrue(t, false)
	_, err := Login(t.Context(), g.client, g.settings, "anna@example.com", "пароль-Анны")
	require.NoError(t, err, "вход не прошёл")
	g.set(func(g *gotrue) { g.busy = 1 })
	keys, err := Unlock(t.Context(), g.client, g.settings, noAsk)
	require.NoError(t, err, "ключи не получены")
	assert.Equal(t, "доступ", keys.Access, "обновление не повторено после перегрузки")
	assert.NoError(t, keys.Down, "причина сбоя после удачного повтора")
}
