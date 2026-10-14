package server

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func TestQueueAcceptsLoginToken(t *testing.T) {
	r := start(t)
	email, password := newEmail(), rand.Text()
	status, body := r.signup(t, email, password)
	require.Equal(t, http.StatusOK, status, "регистрация отклонена: %s", body)
	status, body = r.login(t, email, password)
	require.Equal(t, http.StatusOK, status, "вход отклонён: %s", body)
	s := parse(t, body)
	status, body = r.refresh(t, s.Refresh)
	require.Equal(t, http.StatusOK, status, "продление отклонено: %s", body)
	renewed := parse(t, body).Access

	q := queuetest.New(t)
	js, _ := q.Run(t, false)
	stop, err := access.Serve(t.Context(), js.Conn(), q.Seed, "", "https://"+r.addr, r.client(), q.Server)
	require.NoError(t, err, "служба доступа не получила ключи сервера")
	t.Cleanup(func() { stop() })

	as := func(token string) nats.Option {
		return func(o *nats.Options) error {
			o.Token, o.InboxPrefix = token, nats.InboxPrefix+s.User.ID
			return nil
		}
	}
	_, err = queuetest.JetStream(t, q.Client, as(s.Access)).Publish(t.Context(), record.Topic(s.User.ID), []byte("изменение"))
	assert.NoError(t, err, "очередь не приняла изменение с токеном входа сервера")
	_, err = queuetest.JetStream(t, q.Client, as(renewed)).Publish(t.Context(), record.Topic(s.User.ID), []byte("изменение"))
	assert.NoError(t, err, "очередь не приняла изменение с токеном из продления")
	status, body = r.logout(t, "?scope=local", renewed)
	require.Equal(t, http.StatusNoContent, status, "выход отклонён: %s", body)
	_, err = queuetest.JetStream(t, q.Client, as(renewed)).Publish(t.Context(), record.Topic(s.User.ID), []byte("изменение"))
	assert.NoError(t, err, "очередь не приняла токен завершённой сессии до конца его часа")

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(q.Client.QueueCA)), "CA очереди не разобран")
	forged := s.Access[:len(s.Access)-4] + "AAAA"
	nc, err := nats.Connect(q.Client.Queue, nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}), as(forged))
	if err == nil {
		nc.Close()
	}
	assert.Error(t, err, "очередь пустила токен с подменённой подписью")
}
