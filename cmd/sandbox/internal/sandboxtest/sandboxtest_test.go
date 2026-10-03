package sandboxtest

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomeEnv(t *testing.T) {
	home := t.TempDir()
	snaps.MatchSnapshot(t, strings.ReplaceAll(filepath.ToSlash(strings.Join(HomeEnv(home), "\n")), filepath.ToSlash(home), "<home>"))
}

func startQueue(t *testing.T) (*server.Server, string) {
	cert, key := Cert(t)
	tlsConfig, err := server.GenTLSConfig(&server.TLSConfigOpts{CertFile: cert, KeyFile: key})
	require.NoError(t, err, "TLS очереди не настроен")
	s := natstest.RunServer(&server.Options{
		Host: "127.0.0.1", Port: server.RANDOM_PORT, NoSigs: true, TLSConfig: tlsConfig, TLSTimeout: 3,
		Users: []*server.User{{Username: "client", Password: "client-pass", Permissions: &server.Permissions{
			Subscribe: &server.SubjectPermission{Deny: []string{"forbidden"}},
		}}},
	})
	t.Cleanup(s.Shutdown)
	return s, cert
}

func TestRefusesPlain(t *testing.T) {
	t.Parallel()
	s, _ := startQueue(t)
	RefusesPlain(t, s.Addr().String(), "client", "client-pass")
}

func TestConnect(t *testing.T) {
	t.Parallel()
	s, ca := startQueue(t)
	errs := make(chan error, 1)
	nc, js := Connect(t, s.ClientURL(), ca, errs, nats.UserInfo("client", "client-pass"))
	assert.NotNil(t, js, "JetStream не создан")
	_, err := nc.SubscribeSync("forbidden")
	require.NoError(t, err, "подписка не отправлена")
	require.NoError(t, nc.Flush(), "очередь не ответила")
	select {
	case err := <-errs:
		assert.ErrorIs(t, err, nats.ErrPermissionViolation, "пришла не та ошибка")
	case <-time.After(5 * time.Second):
		assert.Fail(t, "ошибка очереди не дошла до теста")
	}
}
