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
	port, err := FreePort()
	require.NoError(t, err, "свободный порт для очереди устройства не найден")
	q.opts.LeafNode = server.LeafNodeOpts{Host: "127.0.0.1", Port: port, TLSConfig: q.opts.TLSConfig}
	q.Client.QueueLeaf = "tls://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}
