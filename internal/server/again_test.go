package server

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cancelCode = 80877102

type proxy struct {
	addr    string
	mu      sync.Mutex
	until   time.Time
	open    []net.Conn
	refused atomic.Int32
}

func newProxy(t *testing.T, target string) *proxy {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "порт посредника не занят")
	p := &proxy{addr: l.Addr().String()}
	t.Cleanup(func() { _ = l.Close(); p.cut(0) })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go p.pipe(c, target)
		}
	}()
	return p
}

func (p *proxy) pipe(c net.Conn, target string) {
	p.mu.Lock()
	refused := time.Now().Before(p.until)
	p.mu.Unlock()
	if refused {
		head := make([]byte, 8)
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := io.ReadFull(c, head); err == nil && binary.BigEndian.Uint32(head[4:]) != cancelCode {
			p.refused.Add(1)
		}
		_ = c.Close()
		return
	}
	s, err := net.Dial("tcp", target)
	if err != nil {
		_ = c.Close()
		return
	}
	p.mu.Lock()
	p.open = append(p.open, c, s)
	p.mu.Unlock()
	go func() { _, _ = io.Copy(s, c); _ = s.Close() }()
	_, _ = io.Copy(c, s)
	_ = c.Close()
}

func (p *proxy) cut(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.until = time.Now().Add(d)
	for _, c := range p.open {
		_ = c.Close()
	}
	p.open = nil
}

func through(t *testing.T) (*proxy, *running) {
	t.Helper()
	u, err := url.Parse(database(t))
	require.NoError(t, err, "адрес базы не разобран")
	p := newProxy(t, u.Host)
	u.Host = p.addr
	return p, startOn(t, u.String(), tokenKey(t))
}

func TestAgainShortOutage(t *testing.T) {
	p, r := through(t)
	anna := r.user(t)
	r.put(t, anna.User.ID, "почта")
	p.cut(testPauses()[0] + testPauses()[1]/2)
	status, body := r.records(t, listQuery, anna.Access)
	assert.Equal(t, http.StatusOK, status, "список во время короткого сбоя базы отклонён: %s", body)
	assert.Len(t, list(t, body), 1, "список во время сбоя базы неполный")
	p.cut(testPauses()[0] + testPauses()[1]/2)
	status, body = r.refresh(t, anna.Refresh)
	assert.Equal(t, http.StatusOK, status, "продление во время короткого сбоя базы отклонено: %s", body)
	assert.NotEmpty(t, parse(t, body).Access, "продление не выдало токен входа")
	p.cut(0)
	status, body = r.records(t, listQuery, anna.Access)
	assert.Equal(t, http.StatusOK, status, "список после обрыва открытых соединений отклонён: %s", body)
}

func TestAgainLongOutage(t *testing.T) {
	p, r := through(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	anna := parse(t, body)
	p.cut(time.Minute)
	time.Sleep(1100 * time.Millisecond)
	before := p.refused.Load()
	status, body = r.records(t, listQuery, anna.Access)
	assert.Equal(t, answer(http.StatusInternalServerError, internal), answer(status, body), "долгий сбой базы не получил общий отказ")
	assert.EqualValues(t, 3, p.refused.Load()-before, "попыток соединения не три")
	quick := map[string]func() (int, string){
		"регистрация": func() (int, string) { return r.signup(t, newEmail(), password) },
		"вход":        func() (int, string) { return r.login(t, email, password) },
		"выход":       func() (int, string) { return r.logout(t, "?scope=local", anna.Access) },
	}
	for name, send := range quick {
		before := p.refused.Load()
		status, body := send()
		assert.Equal(t, http.StatusInternalServerError, status, "запрос во время сбоя не получил общий отказ: %s: %s", name, body)
		assert.EqualValues(t, 1, p.refused.Load()-before, "запрос повторялся: %s", name)
	}
	client := r.client()
	client.Timeout = testPauses()[0] / 2
	req, err := http.NewRequest(http.MethodGet, "https://"+r.addr+"/rest/v1/records"+listQuery, nil)
	require.NoError(t, err, "запрос не собран")
	req.Header = http.Header{"Apikey": {"app-key"}, "Authorization": {"Bearer " + anna.Access}}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err, "запрос не оборван клиентом")
	at := p.refused.Load()
	time.Sleep(testPauses()[0])
	assert.Equal(t, at, p.refused.Load(), "сервер повторял после обрыва запроса клиентом")
}

func TestAgainOtherError(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	r.exec(t, "ALTER TABLE records RENAME TO records_off")
	began := time.Now()
	status, body := r.records(t, listQuery, anna.Access)
	assert.Equal(t, answer(http.StatusInternalServerError, internal), answer(status, body), "ошибка базы не получила общий отказ")
	assert.Less(t, time.Since(began), testPauses()[0], "ошибка не из списка повторялась")
}

func TestLost(t *testing.T) {
	for err, want := range map[error]bool{
		&pgconn.ConnectError{Config: &pgconn.Config{}}:    true,
		&pgconn.PgError{Code: "08006"}:                    true,
		&pgconn.PgError{Code: "57P01"}:                    true,
		&pgconn.PgError{Code: "57P03"}:                    true,
		io.EOF:                                            true,
		io.ErrUnexpectedEOF:                               true,
		&net.OpError{Op: "read", Err: syscall.ECONNRESET}: true,
		&pgconn.PgError{Code: "42P01"}:                    false,
		&pgconn.PgError{Code: "23505"}:                    false,
		context.Canceled:                                  false,
		context.DeadlineExceeded:                          false,
		&net.OpError{Op: "dial", Err: context.Canceled}:   false,
	} {
		assert.Equal(t, want, lost(fmt.Errorf("запрос: %w", err)), "признак сбоя соединения не тот: %T %v", err, err)
	}
}
