package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

type stored struct {
	ID     string `json:"id"`
	Note   string `json:"note"`
	Secret []byte `json:"secret"`
}

func sealed(t *testing.T, login, password, note string) stored {
	t.Helper()
	return sealedAs(t, map[string]string{"login": login, "password": password}, note)
}

func sealedAs(t *testing.T, fields map[string]string, note string) stored {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	msg, id, err := record.Put(data, annaID, fields, note)
	require.NoError(t, err, "запись не собрана")
	var c record.Change
	require.NoError(t, json.Unmarshal(msg.Data, &c), "тело записи не JSON")
	return stored{ID: id, Note: note, Secret: c.Secret}
}

func serveAs(t *testing.T, access string, reply http.HandlerFunc) *signupServer {
	t.Helper()
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/v1/records") {
			assert.Equal(t, "Bearer "+access, r.Header.Get("Authorization"), "чтение без токена доступа")
			reply(w, r)
			return
		}
		okAs(access)(w, r)
	})
	settingsFile(t, srv.URL, "ключ-приложения")
	return srv
}

func login(t *testing.T) {
	t.Helper()
	_, _, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход не прошёл")
}

func replyJSON(t *testing.T, v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("id") {
			want := strings.TrimPrefix(r.URL.Query().Get("id"), "eq.")
			var found []stored
			for _, s := range v.([]stored) {
				if s.ID == want {
					found = append(found, s)
				}
			}
			v = found
		}
		assert.NoError(t, json.NewEncoder(w).Encode(v), "ответ не записан")
	}
}

var codes = map[string]string{"type": "text", "text": "abcd-1234\nefgh-5678\n\nijkl-9012"}
