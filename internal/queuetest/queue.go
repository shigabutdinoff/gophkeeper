package queuetest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// New готовит очередь на свободном порту, не запуская её.
func New(t testing.TB) *Queue {
	t.Helper()
	web := httptest.NewTLSServer(http.NotFoundHandler())
	defer web.Close()
	url := "tls://" + web.Listener.Addr().String()
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: web.Certificate().Raw}))
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "ключ токенов не создан")
	issuer, err := nkeys.CreateAccount()
	require.NoError(t, err, "ключ выдачи доступа не создан")
	seed, _ := issuer.Seed()
	public, _ := issuer.PublicKey()
	q := &Queue{Client: config.Settings{Queue: url, QueueCA: ca}, Seed: string(seed), key: key,
		Admin: config.Settings{Queue: url, QueueUser: "admin", QueuePass: "admin-pass", QueueCA: ca},
		opts: &server.Options{Host: "127.0.0.1", Port: web.Listener.Addr().(*net.TCPAddr).Port, NoLog: true, NoSigs: true,
			JetStream: true, StoreDir: t.TempDir(), TLSConfig: &tls.Config{Certificates: web.TLS.Certificates, MinVersion: tls.VersionTLS12},
			Users:       []*server.User{{Username: "admin", Password: "admin-pass"}},
			AuthCallout: &server.AuthCallout{Issuer: public, Account: server.DEFAULT_GLOBAL_ACCOUNT, AuthUsers: []string{"admin"}}}}
	q.listenLeaf(t)
	keys := httptest.NewServer(http.HandlerFunc(q.JWKS))
	t.Cleanup(keys.Close)
	q.Supabase = keys.URL
	return q
}
