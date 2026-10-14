package worker

import (
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
)

func TestRunEdits(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"), operation(t, anna, "edit", id, "почта", "n3w-s3cret"))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Equal(t, map[[2]string]string{{anna, id}: "почта/" + b64("n3w-s3cret")}, rows, "правка не применена")
}

func TestRunDeletes(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"), operation(t, anna, "delete", id, "", ""))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Empty(t, rows, "запись не удалена")
	assert.Equal(t, http.MethodDelete, f.writes()[1].Method, "сервер получил не удаление")
}

func TestRunSkipsEditOfDeleted(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{ackWait: 300 * time.Millisecond}
	f := newFake(t)
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"), operation(t, anna, "delete", id, "", ""),
		operation(t, anna, "edit", id, "почта", "n3w-s3cret"))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Empty(t, rows, "удалённая запись вернулась")
	assert.Len(t, f.writes(), 3, "пропущенная правка повторялась")
	assert.Never(t, func() bool {
		info := consumerInfo(t, js)
		return info == nil || info.NumRedelivered > 0 || info.NumAckPending > 0
	}, 3*tune.ackWait, 50*time.Millisecond, "пропущенная правка доставлена повторно")
	skipped := f.log.lines("пропущено")
	for _, l := range f.log.lines("") {
		assert.NotContains(t, l, "n3w-s3cret", "журнал содержит секрет")
		assert.NotContains(t, l, b64("n3w-s3cret"), "журнал содержит секрет")
	}
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(skipped, "\n"), id, "<id>"))
}

func TestRunForeignOwner(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"), operation(t, boris, "edit", id, "взлом", "x"),
		operation(t, boris, "delete", id, "", ""))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("s3cret")}, rows, "запись Анны изменена Борисом")
	assert.Len(t, f.log.lines("пропущено"), 2, "в журнале не два пропуска")
}

func TestRunRetriesMissingTable(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	f := newFake(t)
	f.reply = func(n int, w http.ResponseWriter) bool {
		if n == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"PGRST205"}`))
			return true
		}
		return false
	}
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, anna, id, "s3cret"), operation(t, anna, "delete", id, "", ""))
	run(t, js, f.server())
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Empty(t, rows, "после ответа 404 изменения не применены по порядку")
	assert.Empty(t, f.log.lines("пропущено"), "ответ 404 принят за пропуск")
}

func TestRunSkipsMissingOwner(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	tune := tuning{ackWait: 300 * time.Millisecond}
	f := newFake(t)
	f.reply = func(n int, w http.ResponseWriter) bool {
		if n == 0 {
			w.Header().Set("Proxy-Status", "PostgREST; error=23503")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"23503","message":"s3cret"}`))
		}
		return n == 0
	}
	id := uuid.NewV7().String()
	seq := send(t, q, change(t, boris, id, "s3cret"), change(t, anna, id, "верный"))
	run(t, js, f.server(tune))
	acked(t, js, seq)
	_, rows := f.snapshot()
	assert.Equal(t, map[[2]string]string{{anna, id}: "пометка/" + b64("верный")}, rows, "изменение Анны не применено после удалённого владельца")
	assert.Len(t, f.writes(), 2, "изменение удалённого владельца повторялось")
	assert.Never(t, func() bool {
		info := consumerInfo(t, js)
		return info == nil || info.NumRedelivered > 0 || info.NumAckPending > 0
	}, 3*tune.ackWait, 50*time.Millisecond, "пропущенное изменение доставлено повторно")
	snaps.MatchSnapshot(t, strings.ReplaceAll(strings.Join(f.log.lines("пропущено"), "\n"), id, "<id>"))
}
