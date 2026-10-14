package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

type outage struct {
	mu    sync.Mutex
	until time.Time
	open  []net.Conn
}

func (o *outage) cut(d time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.until = time.Now().Add(d)
	for _, c := range o.open {
		_ = c.Close()
	}
	o.open = nil
}

func (o *outage) pipe(c net.Conn, target string) {
	o.mu.Lock()
	down := time.Now().Before(o.until)
	o.mu.Unlock()
	s, err := net.Dial("tcp", target)
	if down || err != nil {
		_ = c.Close()
		return
	}
	o.mu.Lock()
	o.open = append(o.open, c, s)
	o.mu.Unlock()
	go func() { _, _ = io.Copy(s, c); _ = s.Close() }()
	_, _ = io.Copy(c, s)
	_ = c.Close()
}

func behind(t *testing.T, dsn string) (string, *outage) {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err, "адрес базы не разобран")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт посредника не занят")
	o, target := &outage{}, u.Host
	t.Cleanup(func() { _ = l.Close(); o.cut(0) })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go o.pipe(c, target)
		}
	}()
	u.Host = l.Addr().String()
	return u.String(), o
}

func manyRecords(t *testing.T, n int) []stored {
	t.Helper()
	_, data, err := account.Keys(typedEmail, typedPassword)
	require.NoError(t, err, "ключ к данным не выведен")
	kinds := []map[string]string{{"login": "anna", "password": "s3cret"}, {"type": "text", "text": "abcd-1234\n"}, card()}
	recs := make([]stored, n)
	for i := range recs {
		note := "пометка " + string(rune('а'+i%32))
		msg, id, err := record.Put(data, annaID, kinds[i%len(kinds)], note)
		require.NoError(t, err, "запись не собрана")
		var c record.Change
		require.NoError(t, json.Unmarshal(msg.Data, &c), "тело записи не JSON")
		recs[i] = stored{ID: id, Note: note, Secret: c.Secret}
	}
	slices.SortFunc(recs, func(a, b stored) int { return strings.Compare(a.ID, b.ID) })
	return recs
}

func putOwn(t *testing.T, dsn, owner string, recs []stored) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err, "база не открыта")
	defer func() { _ = conn.Close(context.Background()) }()
	for _, r := range recs {
		_, err = conn.Exec(context.Background(), "INSERT INTO records (id, user_id, note, secret) VALUES ($1, $2, $3, $4)",
			r.ID, owner, r.Note, base64.StdEncoding.EncodeToString(r.Secret))
		require.NoError(t, err, "запись не добавлена")
	}
}

func query(t *testing.T, dsn, sql string, args ...any) string {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err, "база не открыта")
	defer func() { _ = conn.Close(context.Background()) }()
	var got string
	require.NoError(t, conn.QueryRow(context.Background(), sql, args...).Scan(&got), "запрос не выполнен: %s", sql)
	return got
}

func registerOwn(t *testing.T, s config.Settings, dsn string) string {
	t.Helper()
	device(t, s)
	signup(t)
	return query(t, dsn, "SELECT id::text FROM accounts")
}

func listed(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, err := executeIn(keyboard(t, true, ""), append([]string{"list"}, args...)...)
	require.NoError(t, err, "команда list не прошла: %s", errOut)
	return out + errOut
}

func TestOwnRecordsSameAsSupabase(t *testing.T) {
	recs := manyRecords(t, 101)
	s, dsn := startOwnServer(t)
	putOwn(t, dsn, registerOwn(t, s, dsn), recs)
	own, ownOne := listed(t), listed(t, recs[0].ID)
	serveRecords(t, replyJSON(t, recs))
	login(t)
	assert.Equal(t, listed(t), own, "список на своём сервере не тот, что с Supabase")
	serveRecords(t, replyJSON(t, recs))
	login(t)
	assert.Equal(t, listed(t, recs[0].ID), ownOne, "запись на своём сервере не та, что с Supabase")
	shown := 0
	for _, r := range recs {
		if strings.Contains(own, r.ID) {
			shown++
		}
	}
	assert.Equal(t, len(recs), shown, "в списке не все записи")
	assert.NotContains(t, own, "не читается", "запись на своём сервере не прочитана")
}

func TestOwnRecordsForeign(t *testing.T) {
	s, dsn := startOwnServer(t)
	boris := registerOwn(t, s, dsn)
	anna := query(t, dsn, "INSERT INTO accounts (email, encrypted_password) VALUES ('boris@example.com', 'x') RETURNING id::text")
	recs := manyRecords(t, 1)
	putOwn(t, dsn, anna, recs)
	require.NotEqual(t, anna, boris, "у Анны и Бориса одна учётная запись")
	_, _, err := executeIn(keyboard(t, true, ""), "list", recs[0].ID)
	assert.ErrorIs(t, err, account.ErrNotFound, "Борис открыл запись Анны")
	assert.Contains(t, listed(t), "Записей нет.", "список Бориса содержит записи Анны")
}

func TestOwnRecordsFailure(t *testing.T) {
	base := ownDatabase(t)
	dsn, o := behind(t, base)
	s := startOwnOn(t, dsn)
	putOwn(t, base, registerOwn(t, s, base), manyRecords(t, 3))
	before := listed(t)
	o.cut(500 * time.Millisecond)
	assert.Equal(t, before, listed(t), "список при секундном сбое базы не тот")
	rename(t, base, "records", "records_off")
	_, _, err := executeIn(keyboard(t, true, ""), "list")
	assert.ErrorIs(t, err, account.ErrServer, "внутренняя ошибка показана не как у Supabase")
	assert.EqualError(t, err, "сервер вернул ошибку, попробуйте позже")
}
