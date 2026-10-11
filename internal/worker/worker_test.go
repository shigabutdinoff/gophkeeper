package worker

import (
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func TestRunApplies(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	other := nats.NewMsg("changes.other")
	other.Data = []byte("не изменение записи")
	id := uuid.NewV7().String()
	seq := send(t, q, other, change(t, anna, id, "s3cret"))
	run(t, js, f.server())
	acked(t, js, seq)
	calls, rows := f.snapshot()
	require.Len(t, calls, 1, "сервер вызван не один раз")
	assert.Equal(t, []call{
		{Method: http.MethodPost, Table: "records", Owner: anna, Record: id, Note: "пометка", Secret: b64("s3cret"),
			APIKey: "ключ-сервера", Auth: "Bearer ключ-сервера"},
	}, calls, "сервер получил не то изменение")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "запись не сохранена за Анной")
	assert.Equal(t, []string{"обработчик очереди изменений запущен"}, f.log.lines(""), "журнал не тот")
}

func TestRunRetriesServer(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	f.reply = func(n int, w http.ResponseWriter) bool {
		switch n {
		case 0:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 1:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Proxy-Status", "PostgREST; error=PGRST301")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":"PGRST301","message":"s3cret"}`))
		case 2:
			w.Write([]byte("<html>s3cret</html>"))
		case 3:
			w.WriteHeader(http.StatusNoContent)
		case 4:
			conn, _, err := http.NewResponseController(w).Hijack()
			require.NoError(t, err, "соединение не перехвачено")
			require.NoError(t, conn.Close(), "соединение не закрыто")
		case 5:
			w.WriteHeader(http.StatusBadGateway)
		default:
			return false
		}
		return true
	}
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"))
	run(t, js, f.server())
	acked(t, js, seq)
	calls, rows := f.snapshot()
	assert.Len(t, calls, 7, "сервер вызван не семь раз")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "запись не сохранена после повторов")
	assert.Empty(t, f.log.lines("отброшено"), "изменение отброшено при недоступном сервере")
	assert.NotContains(t, f.log.lines(""), "s3cret", "журнал содержит ответ сервера")
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(f.log.lines(""), "\n"), f.URL, "<server>"))
}

func TestRunKeepsOrder(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	f.reply = func(n int, w http.ResponseWriter) bool {
		if n == 0 || n == 2 || n == 4 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		return n == 0 || n == 2 || n == 4
	}
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "первый"), change(t, anna, id, "второй"), change(t, anna, id, "третий"))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	var order []string
	for _, c := range f.writes() {
		order = append(order, c.Secret)
	}
	first, second, third := b64("первый"), b64("второй"), b64("третий")
	assert.Equal(t, []string{first, first, second, second, third, third}, order, "изменения применены не по порядку")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + third}, rows, "осталась не последняя правка")
}

func TestRunRedelivery(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	msg := change(t, anna, id, "s3cret")
	send(t, q, msg)
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	seq := send(t, q, msg)
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Len(t, f.writes(), 2, "повтор не дошёл до сервера")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "повтор создал дубль")
}

func TestRunDropsBroken(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	setVar(t, &ackWait, 300*time.Millisecond)
	f := newFake(t)
	id := uuid.NewV7().String()
	nameless := change(t, anna, id, "s3cret")
	nameless.Subject = "changes.records.anna"
	text := change(t, anna, id, "s3cret")
	text.Data = []byte("s3cret")
	seq := send(t, q, nameless, text, operation(t, anna, "drop", id, "", "s3cret"), noted(t, anna, id, "a\x00", "s3cret"),
		change(t, anna, id, "верный"))
	run(t, js, f.server())
	acked(t, js, seq)
	calls, rows := f.snapshot()
	assert.Len(t, calls, 1, "сервер вызван для неразборчивых изменений")
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("верный")}, rows, "запись Анны изменена неразборчивым изменением")
	assert.Never(t, func() bool {
		info := consumerInfo(t, js)
		return info == nil || info.NumRedelivered > 0 || info.NumAckPending > 0
	}, 3*ackWait, 50*time.Millisecond, "отброшенное изменение доставлено повторно")
	dropped := f.log.lines("отброшено")
	assert.Len(t, dropped, 4, "в журнале не четыре отброшенных изменения")
	for _, l := range f.log.lines("") {
		assert.NotContains(t, l, "s3cret", "журнал содержит секрет")
		assert.NotContains(t, l, b64("s3cret"), "журнал содержит секрет")
	}
	snaps.MatchSnapshot(t, strings.Join(dropped, "\n"))
}

func TestRunAppliesAfterExpiry(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Start(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	client := queuetest.JetStream(t, q.Client, nats.Token(q.Token(t, anna, 2*time.Second)), nats.CustomInboxPrefix("_INBOX."+anna),
		nats.NoReconnect())
	ack, err := client.PublishMsg(t.Context(), change(t, anna, id, "s3cret"))
	require.NoError(t, err, "изменение до срока токена не принято")
	assert.Eventually(t, client.Conn().IsClosed, 5*time.Second, 10*time.Millisecond, "очередь не закрыла соединение после срока токена")
	run(t, js, f.server())
	acked(t, js, ack.Sequence)
	_, rows := f.snapshot()
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "изменение, принятое до срока токена, не применено")
}
