package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"charm.land/huh/v2"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

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

func serveRecords(t *testing.T, reply http.HandlerFunc) *signupServer {
	t.Helper()
	return serveAs(t, "доступ", reply)
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

func TestList(t *testing.T) {
	recs := []stored{sealed(t, "anna", "s3cret", "сайт example.com"), sealed(t, "anna", "pin", ""), sealed(t, "bob", "q", "банк")}
	serveRecords(t, replyJSON(t, recs))
	login(t)
	out, errOut, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	for i, r := range recs {
		out = strings.ReplaceAll(out, r.ID, "<id"+string(rune('1'+i))+">")
	}
	snaps.MatchSnapshot(t, out, errOut)
	assert.NotContains(t, out, "s3cret", "пароль в списке")
	assert.NotContains(t, out, "anna", "логин в списке")
}

func TestListEmpty(t *testing.T) {
	serveRecords(t, replyJSON(t, []stored{}))
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "пустой список не получен")
	assert.Equal(t, "Записей нет.\n", out, "пустой список напечатан не так")
}

func TestListRecord(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "сайт example.com")
	serveRecords(t, replyJSON(t, []stored{rec}))
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list", rec.ID)
	require.NoError(t, err, "запись не получена")
	snaps.MatchSnapshot(t, out)
}

func TestListOtherDevice(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "сайт example.com")
	srv := serveRecords(t, replyJSON(t, []stored{rec}))
	login(t)
	keyring.MockInit()
	settingsFile(t, srv.URL, "ключ-приложения")
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list", rec.ID)
	require.NoError(t, err, "запись не получена на втором устройстве")
	assert.Contains(t, out, "s3cret", "пароль не расшифрован на втором устройстве")
}

func TestListNotFound(t *testing.T) {
	serveRecords(t, replyJSON(t, []stored{}))
	login(t)
	for _, id := range []string{"0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d", "abc"} {
		_, _, err := executeIn(keyboard(t, true, ""), "list", id)
		assert.ErrorIs(t, err, account.ErrNotFound, "нет «запись не найдена» для %q", id)
	}
}

func TestListUnavailable(t *testing.T) {
	srv := serveRecords(t, replyJSON(t, []stored{}))
	login(t)
	srv.Close()
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	assert.Equal(t, account.ErrUnavailable, err, "нет «сервер недоступен»")
}

func TestListBusy(t *testing.T) {
	var n atomic.Int32
	reply := replyJSON(t, []stored{sealed(t, "anna", "s3cret", "")})
	serveRecords(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		reply(w, r)
	})
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "перегрузка не пережита")
	assert.Contains(t, out, "логин и пароль", "список не выведен после повтора")
}

func TestListSilent(t *testing.T) {
	release := make(chan struct{})
	serveRecords(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	login(t)
	start := time.Now()
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	assert.Equal(t, account.ErrNoAnswer, err, "нет «сервер не отвечает»")
	assert.Less(t, time.Since(start), 2*time.Second, "команда ждала дольше срока")
}

func TestListSlowPages(t *testing.T) {
	recs := make([]stored, 201)
	for i := range recs {
		recs[i] = stored{ID: uuid.NewV7().String(), Secret: []byte("не читается")}
	}
	var pages atomic.Int32
	serveRecords(t, func(w http.ResponseWriter, r *http.Request) {
		from := slices.IndexFunc(recs, func(s stored) bool { return "gt."+s.ID == r.URL.Query().Get("id") }) + 1
		pages.Add(1)
		time.Sleep(httpClient.Timeout * 3 / 4)
		assert.NoError(t, json.NewEncoder(w).Encode(recs[from:min(from+100, len(recs))]), "страница не записана")
	})
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "длинный список оборван сроком одной команды")
	assert.EqualValues(t, 3, pages.Load(), "список прочитан не тремя страницами")
	assert.Equal(t, 201, strings.Count(out, "не читается"), "список выведен не целиком")
}

func TestListHidesServerDetails(t *testing.T) {
	serveRecords(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code":"secret-detail","message":"secret-detail"}`))
	})
	login(t)
	for _, args := range [][]string{{"list"}, {"list", "0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d"}} {
		out, errOut, err := executeIn(keyboard(t, true, ""), args...)
		require.ErrorIs(t, err, account.ErrServer, "нет «сервер вернул ошибку» для %q", args)
		assert.NotContains(t, out+errOut+err.Error(), "secret-detail", "текст сервера показан")
	}
}

func TestListSlowPassword(t *testing.T) {
	serveRecords(t, replyJSON(t, []stored{sealed(t, "anna", "s3cret", "")}))
	keyring.MockInitWithError(errors.New("нет хранилища"))
	t.Cleanup(keyring.MockInit)
	login(t)
	in := keyboard(t, true, typedPassword)
	typePassword := runForm
	setVar(t, &runForm, func(c *cobra.Command, f *huh.Form) error {
		time.Sleep(2 * httpClient.Timeout)
		return typePassword(c, f)
	})
	out, _, err := executeIn(in, "list")
	require.NoError(t, err, "медленный ввод пароля оборвал команду")
	assert.Contains(t, out, "логин и пароль", "список не выведен")
}

func TestListArgs(t *testing.T) {
	_, _, err := execute("list", "a", "b")
	assert.EqualError(t, err, `лишний аргумент "b"`, "лишний аргумент принят")
}

func TestListLoggedOut(t *testing.T) {
	serveRecords(t, replyJSON(t, []stored{}))
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	assert.ErrorIs(t, err, account.ErrLoggedOut, "список без входа")
}

var codes = map[string]string{"type": "text", "text": "abcd-1234\nefgh-5678\n\nijkl-9012"}

func TestListTypes(t *testing.T) {
	recs := []stored{sealed(t, "anna", "s3cret", "почта"), sealedAs(t, codes, "GitHub, коды восстановления"), sealedAs(t, card, "Сбербанк, зарплатная")}
	serveRecords(t, replyJSON(t, recs))
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	for i, r := range recs {
		out = strings.ReplaceAll(out, r.ID, "<id"+string(rune('1'+i))+">")
	}
	snaps.MatchSnapshot(t, out)
	for _, secret := range []string{"s3cret", "4111", "123", "abcd", "efgh"} {
		assert.NotContains(t, out, secret, "секрет в списке")
	}
}

func TestListText(t *testing.T) {
	rec := sealedAs(t, codes, "GitHub, коды восстановления")
	serveRecords(t, replyJSON(t, []stored{rec}))
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list", rec.ID)
	require.NoError(t, err, "текст не получен")
	snaps.MatchSnapshot(t, out)
	assert.True(t, strings.HasSuffix(out, "\nТекст:\n"+codes["text"]+"\n"), "текст изменён")
}

func TestListCard(t *testing.T) {
	rec := sealedAs(t, card, "Сбербанк, зарплатная")
	srv := serveRecords(t, replyJSON(t, []stored{rec}))
	login(t)
	keyring.MockInit()
	settingsFile(t, srv.URL, "ключ-приложения")
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list", rec.ID)
	require.NoError(t, err, "карта не получена на втором устройстве")
	snaps.MatchSnapshot(t, out)
}

func TestListBroken(t *testing.T) {
	broken := sealed(t, "anna", "s3cret", "сломана")
	broken.Secret = broken.Secret[:5]
	recs := []stored{sealed(t, "anna", "s3cret", "почта"), broken, sealedAs(t, map[string]string{"type": "photo"}, "фото")}
	serveRecords(t, replyJSON(t, recs))
	login(t)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список с испорченной записью не получен")
	for i, r := range recs {
		assert.Contains(t, out, r.ID, "в списке нет идентификатора записи")
		out = strings.ReplaceAll(out, r.ID, "<id"+string(rune('1'+i))+">")
	}
	snaps.MatchSnapshot(t, out)
	_, _, err = executeIn(keyboard(t, true, ""), "list", broken.ID)
	assert.Error(t, err, "испорченная запись показана")
}
