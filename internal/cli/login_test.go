package cli

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func TestLoginLogout(t *testing.T) {
	keyring.MockInit()
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/logout" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ok(w, r)
	})
	settingsFile(t, srv.URL, "ключ-приложения")
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход не прошёл")
	snaps.MatchSnapshot(t, out, errOut)
	out, errOut, err = execute("logout")
	require.NoError(t, err, "выход не прошёл")
	snaps.MatchSnapshot(t, out, errOut, srv.paths)
	_, _, err = execute("logout")
	assert.NoError(t, err, "повторный выход вернул ошибку")
}

func TestLoginWithoutKeyring(t *testing.T) {
	keyring.MockInitWithError(errors.New("нет хранилища"))
	t.Cleanup(keyring.MockInit)
	srv := startServer(t, ok)
	settingsFile(t, srv.URL, "ключ-приложения")
	out, errOut, err := executeIn(keyboard(t, true, typedPassword), "register")
	require.NoError(t, err, "регистрация не прошла")
	snaps.MatchSnapshot(t, out, errOut)
}

func TestLoginErrors(t *testing.T) {
	keyring.MockInit()
	srv := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error_code":"invalid_credentials"}`))
	})
	settingsFile(t, srv.URL, "ключ-приложения")
	out, _, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.ErrorIs(t, err, account.ErrCredentials, "неверный пароль не распознан")
	assert.Empty(t, out, "при ошибке напечатан stdout")
	_, _, err = executeIn(keyboard(t, false, typedPassword), "login")
	require.Error(t, err, "вход без клавиатуры прошёл")
	settingsFile(t, "http://example.invalid", "к")
	_, _, err = executeIn(keyboard(t, true, typedPassword), "login")
	require.Error(t, err, "вход прошёл по http")
	assert.Equal(t, int32(1), srv.requests.Load(), "запросы ушли на сервер")
}
