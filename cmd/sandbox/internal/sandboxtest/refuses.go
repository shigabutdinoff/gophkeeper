package sandboxtest

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RefusesPlain проверяет, что очередь по адресу addr требует TLS и не
// обслуживает пользователя с верным паролем без шифрования.
func RefusesPlain(t testing.TB, addr, user, password string) {
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
	_, err = fmt.Fprintf(conn, "CONNECT {\"user\":%q,\"pass\":%q}\r\nPING\r\n", user, password)
	require.NoError(t, err, "команда CONNECT не отправлена")
	rest, err := io.ReadAll(r)
	require.NoError(t, err, "очередь не закрыла соединение без TLS")
	assert.NotContains(t, string(rest), "PONG", "очередь обслужила клиента без TLS")
}
