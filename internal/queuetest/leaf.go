package queuetest

import (
	"net"
	"strconv"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

func (q *Queue) listenLeaf(t testing.TB) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "свободный порт для очереди устройства не найден")
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close(), "порт для очереди устройства не освобождён")
	q.opts.LeafNode = server.LeafNodeOpts{Host: "127.0.0.1", Port: port, TLSConfig: q.opts.TLSConfig}
	q.Client.QueueLeaf = "tls://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}
