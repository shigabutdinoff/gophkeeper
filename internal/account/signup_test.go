package account

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func TestSignup(t *testing.T) {
	cases := map[string]struct {
		reply http.HandlerFunc
		want  error
	}{
		"created": {func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"refresh_token":"обновление"}`))
		}, nil},
		"exists": {func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"code":422,"error_code":"user_already_exists","msg":"User already registered"}`))
		}, ErrExists},
		"unknown": {func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error_code":"secret-detail","msg":"secret-detail"}`))
		}, ErrServer},
		"not json": {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, ErrServer},
		"silent": {func(_ http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
		}, ErrNoAnswer},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewTLSServer(c.reply)
			t.Cleanup(srv.Close)
			client := srv.Client()
			client.Timeout = 200 * time.Millisecond
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			keyring.MockInit()
			saved, err := Signup(t.Context(), client, config.Settings{Server: srv.URL + "/", AppKey: "ключ"}, "anna@example.com", "пароль-Анны")
			if c.want == nil {
				assert.NoError(t, err, "регистрация вернула ошибку")
				assert.True(t, saved, "вход после регистрации не сохранён")
				return
			}
			assert.ErrorIs(t, err, c.want, "регистрация вернула не ту ошибку")
			snaps.MatchSnapshot(t, strings.ReplaceAll(err.Error(), srv.URL, "$SERVER"))
		})
	}
}

func TestSignupBadServer(t *testing.T) {
	_, err := Signup(t.Context(), nil, config.Settings{Server: "https://a b"}, "anna@example.com", "")
	require.Error(t, err, "регистрация прошла с неверным адресом")
	snaps.MatchSnapshot(t, err.Error())
}
