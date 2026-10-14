package access_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const (
	anna  = "0199b5e4-0000-7000-8000-00000000a11a"
	boris = "0199b5e4-0000-7000-8000-0000000b0715"
)

func TestMain(m *testing.M) {
	if (snapstest.CleanM{M: m}).Run() != 0 {
		log.Fatal("тесты пакета или снапшоты не прошли проверку")
	}
}

func dial(t *testing.T, q *queuetest.Queue, opts ...nats.Option) (*nats.Conn, error) {
	t.Helper()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(q.Client.QueueCA)), "CA очереди не разобран")
	nc, err := nats.Connect(q.Client.Queue, append([]nats.Option{nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})}, opts...)...)
	if err == nil {
		t.Cleanup(nc.Close)
	}
	return nc, err
}

func claims(q *queuetest.Queue, owner string, ttl time.Duration) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{Issuer: q.Supabase + "/auth/v1", Subject: owner,
		Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl))}
}

func TestRefuses(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	foreign, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "чужой ключ не создан")
	signed := func(method jwt.SigningMethod, key any, c jwt.RegisteredClaims) string {
		token, err := jwt.NewWithClaims(method, c).SignedString(key)
		require.NoError(t, err, "токен не подписан")
		return token
	}
	edit := func(change func(*jwt.RegisteredClaims)) string {
		c := claims(q, anna, time.Hour)
		change(&c)
		return q.Sign(t, c)
	}
	cases := []struct{ name, token string }{
		{"без токена", ""}, {"мусор", "s3cret"},
		{"истёк", q.Token(t, anna, -time.Minute)},
		{"чужой сервер", edit(func(c *jwt.RegisteredClaims) { c.Issuer = "https://example.com/auth/v1" })},
		{"чужая аудитория", edit(func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"anon"} })},
		{"без срока", edit(func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil })},
		{"пользователь не UUID", edit(func(c *jwt.RegisteredClaims) { c.Subject = "*" })},
		{"пользователь с маской", edit(func(c *jwt.RegisteredClaims) { c.Subject = anna + ".>" })},
		{"общий секрет", signed(jwt.SigningMethodHS256, []byte("s3cret"), claims(q, anna, time.Hour))},
		{"чужой ключ", signed(jwt.SigningMethodES256, foreign, claims(q, anna, time.Hour))},
	}
	for _, c := range cases {
		_, err := dial(t, q, nats.Token(c.token))
		assert.ErrorIs(t, err, nats.ErrAuthorization, "очередь пустила: %s", c.name)
	}
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	assert.Zero(t, info.State.Msgs, "в потоке появилось изменение")
}

func TestOwnerOnly(t *testing.T) {
	q := queuetest.New(t)
	_, stream := q.Start(t, false)
	errs := make(chan string, 32)
	nc, err := dial(t, q, q.As(t, anna), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		errs <- err.Error()
	}))
	require.NoError(t, err, "вошедшее устройство не подключилось")
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	_, err = js.Publish(t.Context(), record.Topic(anna), []byte("своё"))
	require.NoError(t, err, "изменение в свою тему не принято")
	for _, subject := range []string{record.Topic(boris), record.Subject, "changes.files." + anna, "$SYS.REQ.USER.AUTH", "$JS.API.STREAM.INFO.CHANGES"} {
		require.NoError(t, nc.Publish(subject, []byte("чужое")), "публикация в %s не отправлена", subject)
	}
	for _, subject := range []string{"changes.>", record.Topic(anna), "$SYS.REQ.USER.AUTH", "_INBOX." + boris + ".>", ">"} {
		_, err := nc.SubscribeSync(subject)
		require.NoError(t, err, "подписка на %s не отправлена", subject)
	}
	require.NoError(t, nc.Flush(), "очередь не ответила")
	var refused []string
	for len(refused) < 10 {
		select {
		case reason := <-errs:
			refused = append(refused, reason)
		case <-time.After(2 * time.Second):
			t.Fatalf("отказов не десять: %v", refused)
		}
	}
	snaps.MatchSnapshot(t, refused)
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	assert.Equal(t, uint64(1), info.State.Msgs, "в потоке не одно изменение")
}

func TestExpiryCloses(t *testing.T) {
	q := queuetest.New(t)
	q.Start(t, false)
	closed := make(chan error, 8)
	nc, err := dial(t, q, nats.Token(q.Token(t, anna, 2*time.Second)), nats.CustomInboxPrefix("_INBOX."+anna), nats.NoReconnect(),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { closed <- err }))
	require.NoError(t, err, "устройство с действующим токеном не подключилось")
	require.NoError(t, nc.Publish(record.Topic(anna), []byte("своё")), "изменение до срока не отправлено")
	select {
	case err := <-closed:
		assert.ErrorIs(t, err, nats.ErrAuthExpired, "соединение закрыто не из-за срока токена")
	case <-time.After(5 * time.Second):
		t.Fatal("очередь не закрыла соединение после срока токена")
	}
}

func TestServeBadSeed(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	_, err := access.Serve(t.Context(), js.Conn(), "s3cret", "", q.Supabase, http.DefaultClient, q.Server)
	require.Error(t, err, "служба запущена с неверным ключом выдачи")
	assert.NotContains(t, err.Error(), "s3cret", "в ошибке ключ выдачи")
	snaps.MatchSnapshot(t, err.Error())
	assert.True(t, errors.Unwrap(err) != nil, "причина ошибки потеряна")
}

func TestServeWithoutKeys(t *testing.T) {
	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	stop, err := access.Serve(t.Context(), js.Conn(), q.Seed, "", "http://127.0.0.1:1", http.DefaultClient, q.Server)
	require.NoError(t, err, "служба не запустилась, пока сервер недоступен")
	defer func() { _ = stop() }()
	_, err = dial(t, q, q.As(t, anna))
	assert.ErrorIs(t, err, nats.ErrAuthorization, "очередь пустила устройство без ключей сервера")
}
