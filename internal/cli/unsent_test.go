package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func queueDown(t *testing.T, recs ...stored) (*signupServer, *queuetest.Queue) {
	t.Helper()
	q := queuetest.New(t)
	srv := serveAs(t, q.Token(t, annaID, time.Hour), replyJSON(t, recs))
	queueSettings(t, srv.URL, q.Client)
	login(t)
	return srv, q
}

func annaData(t *testing.T) string {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	return data
}

func onDevice(t *testing.T, q *queuetest.Queue, msgs ...*nats.Msg) {
	t.Helper()
	o, err := queue.Open(t.Context(), q.Client, annaID, annaData(t), "")
	require.NoError(t, err, "очередь устройства не открыта")
	defer o.Close()
	for _, msg := range msgs {
		require.NoError(t, o.Put(t.Context(), msg), "изменение не сохранено на устройстве")
	}
}

func unsentPut(t *testing.T, data string, fields map[string]string, note string) (*nats.Msg, string) {
	t.Helper()
	msg, id, err := record.Put(data, annaID, fields, note)
	require.NoError(t, err, "запись не собрана")
	return msg, id
}

func noServer(t *testing.T, password string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	out, _, err := executeCtx(ctx, keyboard(t, true, password), args...)
	return out, err
}

func TestListUnsentOffline(t *testing.T) {
	srv, _ := queueDown(t)
	_, err := noServer(t, "s3cret", "add", "--login", "anna", "--note", "сайт example.com")
	require.NoError(t, err, "запись без очереди не сохранена на устройстве")
	srv.Close()
	out, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	id := recordID.FindString(out)
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"))
	out, err = noServer(t, "", "list", id)
	require.NoError(t, err, "неотправленная запись не показана")
	snaps.MatchSnapshot(t, out)
}

func TestListUnsentOnline(t *testing.T) {
	first := sealed(t, "anna", "s3cret", "почта")
	msg, id := unsentPut(t, annaData(t), map[string]string{"login": "anna", "password": "new"}, "новая")
	last := sealed(t, "bob", "q", "банк")
	_, q := queueDown(t, first, last)
	onDevice(t, q, msg)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	for i, r := range []string{first.ID, id, last.ID} {
		out = strings.ReplaceAll(out, r, "<id"+string(rune('1'+i))+">")
	}
	snaps.MatchSnapshot(t, out)
}

func TestListUnsentEdit(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "старая")
	_, q := queueDown(t, rec)
	msg, err := record.Edit(annaData(t), annaID, rec.ID, map[string]string{"login": "anna", "password": "new"}, "правка")
	require.NoError(t, err, "правка не собрана")
	onDevice(t, q, msg)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	assert.Equal(t, 1, strings.Count(out, rec.ID), "запись показана не один раз")
	assert.Contains(t, out, "логин и пароль, не отправлено", "нет отметки у правки")
	assert.Contains(t, out, "правка", "нет новой пометки")
	assert.NotContains(t, out, "старая", "показана старая пометка")
}

func TestListUnsentDelete(t *testing.T) {
	kept, gone := sealed(t, "anna", "s3cret", "остаётся"), sealed(t, "bob", "q", "удалить")
	srv, q := queueDown(t, kept, gone)
	msg, err := record.Delete(annaID, gone.ID)
	require.NoError(t, err, "удаление не собрано")
	onDevice(t, q, msg)
	out, _, err := executeIn(keyboard(t, true, ""), "list")
	require.NoError(t, err, "список не получен")
	assert.Contains(t, out, kept.ID, "нет оставшейся записи")
	assert.NotContains(t, out, gone.ID, "удалённая запись показана")
	_, _, err = executeIn(keyboard(t, true, ""), "list", gone.ID)
	assert.ErrorIs(t, err, account.ErrNotFound, "удалённая запись найдена")
	srv.Close()
	_, err = noServer(t, "", "list", gone.ID)
	assert.ErrorIs(t, err, account.ErrNotFound, "удалённая запись найдена без сервера")
}

func TestListUnsentOnlyDelete(t *testing.T) {
	srv, q := queueDown(t)
	msg, err := record.Delete(annaID, missing)
	require.NoError(t, err, "удаление не собрано")
	onDevice(t, q, msg)
	srv.Close()
	_, err = noServer(t, "", "list")
	assert.Equal(t, account.ErrUnavailable, err, "нет «сервер недоступен»")
}

func TestListUnsentOther(t *testing.T) {
	srv, q := queueDown(t)
	msg, _ := unsentPut(t, annaData(t), codes, "коды")
	onDevice(t, q, msg)
	srv.Close()
	_, err := noServer(t, "", "list", missing)
	assert.Equal(t, account.ErrUnavailable, err, "нет «сервер недоступен» для другой записи")
	_, err = noServer(t, "", "list", "нечто")
	assert.ErrorIs(t, err, account.ErrNotFound, "неверный идентификатор принят")
}

func TestListUnsentText(t *testing.T) {
	srv, q := queueDown(t)
	msg, id := unsentPut(t, annaData(t), codes, "GitHub, коды восстановления")
	onDevice(t, q, msg)
	srv.Close()
	out, err := noServer(t, "", "list", id)
	require.NoError(t, err, "неотправленный текст не показан")
	snaps.MatchSnapshot(t, out)
}

func TestListUnsentForeign(t *testing.T) {
	srv, q := queueDown(t)
	_, foreign, err := account.Keys("boris@example.com", typedPassword)
	require.NoError(t, err, "чужой ключ не выведен")
	msg, id := unsentPut(t, foreign, map[string]string{"login": "boris", "password": "x"}, "чужая")
	onDevice(t, q, msg)
	srv.Close()
	out, err := noServer(t, "", "list")
	require.NoError(t, err, "список без сервера не показан")
	snaps.MatchSnapshot(t, strings.ReplaceAll(out, id, "<id>"))
}
