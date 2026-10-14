package queuetest

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// JetStream подключается к очереди с настройками s и параметрами extra.
func JetStream(t testing.TB, s config.Settings, extra ...nats.Option) jetstream.JetStream {
	t.Helper()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(s.QueueCA)), "CA очереди не разобран")
	nc, err := nats.Connect(s.Queue, append([]nats.Option{nats.UserInfo(s.QueueUser, s.QueuePass),
		nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})}, extra...)...)
	require.NoError(t, err, "очередь не приняла подключение")
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	return js
}
