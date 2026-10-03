package sandboxtest

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

// Connect подключается к очереди url с доверием к ca и закрывает соединение
// после теста. Асинхронные ошибки уходят в errs, пока в нём есть место.
func Connect(t testing.TB, url, ca string, errs chan<- error, opts ...nats.Option) (*nats.Conn, jetstream.JetStream) {
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
