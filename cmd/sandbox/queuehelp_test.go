package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func connect(t testing.TB, url, ca string, errs chan<- error, opts ...nats.Option) (*nats.Conn, jetstream.JetStream) {
	opts = append([]nats.Option{nats.RootCAs(ca), nats.ErrorHandler(sendErr(errs))}, opts...)
	nc, err := nats.Connect(url, opts...)
	require.NoError(t, err, "очередь %q не приняла подключение", url)
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream очереди недоступен")
	return nc, js
}

func sendErr(errs chan<- error) nats.ErrHandler {
	return func(_ *nats.Conn, _ *nats.Subscription, err error) {
		select {
		case errs <- err:
		default:
		}
	}
}

func refusesPlain(t testing.TB, addr, password string) {
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err, "порт очереди недоступен")
	t.Cleanup(func() {
		assert.NoError(t, conn.Close(), "соединение с очередью не закрыто")
	})
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)), "срок ответа не задан")
	r := bufio.NewReader(conn)
	info, err := r.ReadString('\n')
	require.NoError(t, err, "очередь не прислала INFO")
	assert.Contains(t, info, `"tls_required":true`, "очередь не требует TLS")
	_, err = fmt.Fprintf(conn, "CONNECT {\"user\":%q,\"pass\":%q}\r\nPING\r\n", clientUser, password)
	require.NoError(t, err, "команда CONNECT не отправлена")
	rest, err := io.ReadAll(r)
	require.NoError(t, err, "очередь не закрыла соединение без TLS")
	assert.NotContains(t, string(rest), "PONG", "очередь обслужила клиента без TLS")
}
