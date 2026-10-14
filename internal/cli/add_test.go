package cli

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

var recordID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`)

func queueSettings(t *testing.T, server string, q config.Settings) {
	t.Helper()
	q.Server, q.AppKey = server, "ключ-приложения"
	writeSettings(t, q)
}

func loggedIn(t *testing.T, q config.Settings, access string) *signupServer {
	t.Helper()
	keyring.MockInit()
	srv := startServer(t, okAs(access))
	queueSettings(t, srv.URL, q)
	login(t)
	return srv
}

func TestAdd(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	out, errOut, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna", "--note", "сайт example.com")
	require.NoError(t, err, "запись не принята")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), errOut)
	assert.Contains(t, out, lastChange(t, stream).Record, "напечатан не тот идентификатор записи")
	assert.Equal(t, uint64(1), queued(t, stream), "в очереди не одно изменение")
}

func TestAddWithoutNote(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	_, errOut, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.NoError(t, err, "запись без пометки не принята")
	snaps.MatchSnapshot(t, errOut)
}

func TestAddOffline(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	srv := loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	srv.Close()
	out, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.NoError(t, err, "запись без сервера не принята")
	assert.Contains(t, out, lastChange(t, stream).Record, "напечатан не тот идентификатор записи")
}

func TestAddRefusesBeforeSecrets(t *testing.T) {
	q := queuetest.New(t).Client
	plain := q
	plain.Queue = "nats" + q.Queue[len("tls"):]
	cases := map[string]struct {
		settings config.Settings
		args     []string
		terminal bool
	}{
		"insecure":     {plain, []string{"--login", "anna"}, true},
		"no login":     {q, nil, true},
		"not terminal": {q, []string{"--login", "anna"}, false},
		"bogus type":   {q, []string{"--type", "bogus"}, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			loggedIn(t, c.settings, "доступ")
			in := keyboard(t, c.terminal, "s3cret")
			setVar(t, &runForm, func(*cobra.Command, *huh.Form) error { return errors.New("пароль спрошен") })
			out, _, err := executeIn(in, append([]string{"add"}, c.args...)...)
			require.Error(t, err, "запись принята")
			assert.Empty(t, out, "при ошибке напечатано подтверждение")
			snaps.MatchSnapshot(t, err.Error())
		})
	}
}

func TestAddText(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	srv := loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	srv.Close()
	keyboard(t, false)
	out, errOut, err := executeIn(input(t, codes()["text"]), "add", "--type", "text", "--login", "x", "--note", "GitHub, коды восстановления")
	require.NoError(t, err, "текст без сервера не принят")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), errOut)
	c := lastChange(t, stream)
	assert.Equal(t, []string{"put", "GitHub, коды восстановления"}, []string{c.Op, c.Note}, "в очереди не та запись")
	assert.Equal(t, codes(), secretOf(t, c), "текст изменён")
}

func TestAddCard(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	typing(t, "4111 1111 1111 1111", "12/28", "ANNA IVANOVA", "123")
	_, errOut, err := executeIn(input(t, ""), "add", "--type", "card", "--note", "Сбербанк, зарплатная")
	require.NoError(t, err, "карта не принята")
	snaps.MatchSnapshot(t, errOut)
	assert.Equal(t, card(), secretOf(t, lastChange(t, stream)), "не та карта")
}

func TestAddLoginSecret(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	loggedIn(t, q.Client, q.Token(t, annaID, time.Hour))
	_, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.NoError(t, err, "логин не принят")
	assert.Equal(t, map[string]string{"login": "anna", "password": "s3cret"}, secretOf(t, lastChange(t, stream)), "не тот секрет логина")
}

func TestAddOfflineExpired(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	for _, access := range []string{q.Token(t, annaID, -time.Minute), ""} {
		srv := loggedIn(t, q.Client, access)
		srv.Close()
		out, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
		require.NoError(t, err, "запись с истёкшим токеном не сохранена на устройстве")
		snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"))
		assert.True(t, queue.Kept(), "запись не осталась на устройстве")
	}
	assert.Zero(t, queued(t, stream), "запись с истёкшим токеном ушла в очередь")
}

func TestAddDenied(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	loggedIn(t, q.Client, q.Token(t, annaID, -time.Minute))
	out, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.ErrorIs(t, err, queue.ErrDenied, "отказ очереди при доступном сервере не распознан")
	assert.Empty(t, out, "при отказе напечатано подтверждение")
	snaps.MatchSnapshot(t, err.Error())
	assert.Zero(t, queued(t, stream), "запись без доступа ушла в очередь")
	assert.True(t, queue.Kept(), "запись без доступа не осталась на устройстве")
}
