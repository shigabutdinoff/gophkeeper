package cli

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const missing = "0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d"

func withQueue(t *testing.T, reply http.HandlerFunc) (*signupServer, jetstream.Stream) {
	t.Helper()
	return queueFor(t, time.Hour, reply)
}

func queueFor(t *testing.T, ttl time.Duration, reply http.HandlerFunc) (*signupServer, jetstream.Stream) {
	t.Helper()
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	srv := serveAs(t, q.Token(t, annaID, ttl), reply)
	queueSettings(t, srv.URL, q.Client)
	login(t)
	return srv, stream
}

func lastChange(t *testing.T, stream jetstream.Stream) record.Change {
	t.Helper()
	msg, err := stream.GetLastMsgForSubject(t.Context(), record.Topic(annaID))
	require.NoError(t, err, "изменения нет в очереди")
	c, err := record.Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "изменение не разобрано")
	return c
}

func queued(t *testing.T, stream jetstream.Stream) uint64 {
	t.Helper()
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	return info.State.Msgs
}

func opened(t *testing.T, c record.Change) []string {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	fields, err := record.Open(data, c.Record, c.Secret)
	require.NoError(t, err, "секрет правки не открыт")
	return []string{fields["login"], fields["password"], c.Note}
}

func secretOf(t *testing.T, c record.Change) map[string]string {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	fields, err := record.Open(data, c.Record, c.Secret)
	require.NoError(t, err, "секрет правки не открыт")
	return fields
}

func TestEdit(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "сайт example.com")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	out, _, err := executeIn(keyboard(t, true, "n3w-s3cret"), "edit", rec.ID)
	require.NoError(t, err, "правка не принята")
	assert.Equal(t, "Изменение принято, запись "+rec.ID+".\n", out, "не тот ответ")
	c := lastChange(t, stream)
	assert.Equal(t, []string{"edit", rec.ID}, []string{c.Op, c.Record}, "в очереди не та правка")
	assert.Equal(t, []string{"anna", "n3w-s3cret", "сайт example.com"}, opened(t, c), "логин или пометка не прежние")
}

func TestEditFile(t *testing.T) {
	file := map[string]string{"type": "file", "name": "id_ed25519", "size": "411"}
	rec := sealedAs(t, file, "ключ SSH для рабочего сервера")
	srv, stream := withQueue(t, replyJSON(t, []stored{rec}))
	before := srv.requests.Load()
	_, _, err := executeIn(keyboard(t, true, ""), "edit", rec.ID, "--file", "./new")
	require.EqualError(t, err, "чтобы заменить файл, удалите запись и добавьте файл заново", "замена содержимого принята")
	assert.Equal(t, before, srv.requests.Load(), "сервер опрошен при замене содержимого")
	assert.Zero(t, queued(t, stream), "замена содержимого ушла в очередь")

	keyboard(t, true, "")
	setVar(t, &runForm, func(*cobra.Command, *huh.Form) error { return errors.New("поля файла спрошены") })
	out, _, err := executeIn(input(t, ""), "edit", rec.ID, "--note", "ключ SSH, старый сервер")
	require.NoError(t, err, "пометка файла не изменена")
	assert.Equal(t, "Изменение принято, запись "+rec.ID+".\n", out, "не тот ответ")
	c := lastChange(t, stream)
	assert.Equal(t, []string{"edit", rec.ID, "ключ SSH, старый сервер", ""}, []string{c.Op, c.Record, c.Note, c.Digest}, "в очереди не та правка")
	assert.Equal(t, file, secretOf(t, c), "имя или размер файла не прежние")

	_, _, err = executeIn(keyboard(t, true, ""), "delete", rec.ID)
	require.NoError(t, err, "удаление файла не принято")
	c = lastChange(t, stream)
	assert.Equal(t, []string{"delete", rec.ID}, []string{c.Op, c.Record}, "в очереди не удаление файла")
}

func TestEditFlags(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "сайт example.com")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	_, _, err := executeIn(keyboard(t, true, "n3w-s3cret"), "edit", rec.ID, "--login", "bob", "--note", "")
	require.NoError(t, err, "правка с флагами не принята")
	assert.Equal(t, []string{"bob", "n3w-s3cret", ""}, opened(t, lastChange(t, stream)), "флаги не применены")
}

func TestEditNotFound(t *testing.T) {
	_, stream := withQueue(t, replyJSON(t, []stored{}))
	for _, args := range [][]string{{"edit", missing}, {"delete", missing}, {"edit", "abc"}} {
		_, errOut, err := executeIn(keyboard(t, true, "n3w-s3cret"), args...)
		assert.ErrorIs(t, err, account.ErrNotFound, "нет «запись не найдена» для %q", args)
		assert.NotContains(t, errOut, "Пароль записи", "пароль спрошен для чужой записи")
	}
	assert.Zero(t, queued(t, stream), "чужая запись ушла в очередь")
}

func TestEditOffline(t *testing.T) {
	srv, stream := withQueue(t, replyJSON(t, []stored{}))
	srv.Close()
	for _, args := range [][]string{{"edit", missing}, {"edit", missing, "--login", "anna", "--note", "x"}} {
		out, errOut, err := executeIn(keyboard(t, true, "n3w-s3cret"), args...)
		assert.ErrorIs(t, err, account.ErrUnavailable, "правка без сервера принята: %q", args)
		assert.Empty(t, out, "при отказе напечатано подтверждение: %q", args)
		assert.NotContains(t, errOut, "Пароль записи", "пароль спрошен без сервера: %q", args)
	}
	assert.Zero(t, queued(t, stream), "правка без сервера ушла в очередь")
}

func TestEditServerError(t *testing.T) {
	_, stream := withQueue(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	for _, args := range [][]string{{"edit", missing}, {"delete", missing}} {
		_, _, err := executeIn(keyboard(t, true, "n3w-s3cret"), args...)
		assert.ErrorIs(t, err, account.ErrServer, "нет «сервер вернул ошибку» для %q", args)
	}
	assert.Zero(t, queued(t, stream), "изменение ушло в очередь при ошибке сервера")
}

func TestEditEmptyPassword(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "")
	withQueue(t, replyJSON(t, []stored{rec}))
	_, _, err := executeIn(keyboard(t, true, ""), "edit", rec.ID)
	assert.EqualError(t, err, "пароль записи не может быть пустым", "пустой пароль принят")
	_, _, err = executeIn(keyboard(t, false, "n3w-s3cret"), "edit", rec.ID)
	assert.EqualError(t, err, "пароль вводится только с клавиатуры", "пароль не с клавиатуры принят")
}

func TestEditArgs(t *testing.T) {
	for _, cmd := range []string{"edit", "delete"} {
		_, _, err := execute(cmd)
		assert.EqualError(t, err, "укажите идентификатор записи", "%s без идентификатора принят", cmd)
		_, _, err = execute(cmd, "a", "b")
		assert.EqualError(t, err, `лишний аргумент "b"`, "%s с лишним аргументом принят", cmd)
	}
}

func TestTooLargeNotQueued(t *testing.T) {
	rec := sealed(t, "anna", "s3cret", "")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	huge := strings.Repeat("я", 40000)
	_, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna", "--note", huge)
	assert.ErrorIs(t, err, record.ErrTooLarge, "огромная пометка принята в add")
	_, _, err = executeIn(keyboard(t, true, "s3cret"), "edit", rec.ID, "--note", huge)
	assert.ErrorIs(t, err, record.ErrTooLarge, "огромная пометка принята в edit")
	assert.Zero(t, queued(t, stream), "огромное изменение ушло в очередь")
}

func TestEditCard(t *testing.T) {
	rec := sealedAs(t, card(), "Сбербанк, зарплатная")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	typing(t, "", "01/30", "", "")
	_, _, err := executeIn(input(t, ""), "edit", rec.ID)
	require.NoError(t, err, "правка карты не принята")
	c := lastChange(t, stream)
	assert.Equal(t, []string{"edit", rec.ID, "Сбербанк, зарплатная"}, []string{c.Op, c.Record, c.Note}, "в очереди не та правка")
	assert.Equal(t, map[string]string{"type": "card", "number": "4111 1111 1111 1111", "expiry": "01/30", "holder": "ANNA IVANOVA", "code": "123"}, secretOf(t, c), "прежние поля карты не сохранены")
	_, _, err = executeIn(input(t, ""), "edit", rec.ID, "--type", "text")
	assert.Error(t, err, "флаг типа принят в edit")
}

func TestEditText(t *testing.T) {
	rec := sealedAs(t, codes(), "GitHub")
	_, stream := withQueue(t, replyJSON(t, []stored{rec}))
	keyboard(t, false)
	_, _, err := executeIn(input(t, ""), "edit", rec.ID)
	require.NoError(t, err, "правка текста не принята")
	assert.Equal(t, codes(), secretOf(t, lastChange(t, stream)), "прежний текст не сохранён")
	_, _, err = executeIn(input(t, "новый\n"), "edit", rec.ID, "--note", "")
	require.NoError(t, err, "новый текст не принят")
	c := lastChange(t, stream)
	assert.Equal(t, map[string]string{"type": "text", "text": "новый\n"}, secretOf(t, c), "не тот новый текст")
	assert.Empty(t, c.Note, "пометка не очищена")
}

func TestDeleteOfflineExpired(t *testing.T) {
	srv, stream := queueFor(t, -time.Minute, replyJSON(t, []stored{}))
	srv.Close()
	out, _, err := executeIn(keyboard(t, true, "n3w-s3cret"), "delete", missing)
	require.NoError(t, err, "удаление с истёкшим токеном не сохранено на устройстве")
	assert.Contains(t, out, "сохранено на устройстве", "не сказано, что удаление ждёт отправки")
	assert.Zero(t, queued(t, stream), "удаление с истёкшим токеном ушло в очередь")
}
