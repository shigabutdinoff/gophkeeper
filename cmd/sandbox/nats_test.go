package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/shigabutdinoff/gophkeeper/cmd/sandbox/internal/sandboxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type queue struct {
	url, addr, ca, user, inbox string
	admin                      jetstream.JetStream
}

func startQueue(t *testing.T) queue {
	cert, key := sandboxtest.Cert(t)
	dir := t.TempDir()
	conf := strings.NewReplacer("$$", "$",
		"/etc/nats/sandbox/cert.pem", strconv.Quote(cert), "/etc/nats/sandbox/key.pem", strconv.Quote(key),
	).Replace(parseModel(t).Configs["nats"].Content)
	path := filepath.Join(dir, "nats.conf")
	require.NoError(t, os.WriteFile(path, []byte(conf), 0o600), "конфиг очереди не записан")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sandbox"), 0o700), "каталог секретов не создан")
	passwords := "ADMIN_PASSWORD: \"admin-pass\"\nCLIENT_PASSWORD: \"client-pass\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sandbox", "passwords.conf"), []byte(passwords), 0o600), "пароли не записаны")
	s, _ := natstest.RunServerWithConfigOverrides(path, func(o *server.Options) {
		o.Host, o.Port, o.HTTPPort, o.StoreDir, o.NoSigs = "127.0.0.1", server.RANDOM_PORT, 0, t.TempDir(), true
	}, nil)
	t.Cleanup(s.Shutdown)
	q := queue{url: s.ClientURL(), addr: s.Addr().String(), ca: cert, user: "client", inbox: queueInbox}
	_, q.admin = sandboxtest.Connect(t, q.url, q.ca, nil, nats.UserInfo("admin", "admin-pass"), nats.CustomInboxPrefix("_INBOX.adm"))
	stream, err := loadStream()
	require.NoError(t, err, "поток из модели не разбирается")
	_, err = q.admin.CreateStream(context.Background(), stream)
	require.NoError(t, err, "поток CHANGES не создан")
	return q
}

func (q queue) client(t *testing.T, errs chan<- error, opts ...nats.Option) (*nats.Conn, jetstream.JetStream) {
	return sandboxtest.Connect(t, q.url, q.ca, errs, append(opts, nats.UserInfo(q.user, "client-pass"), nats.CustomInboxPrefix(q.inbox))...)
}

func TestQueueClient(t *testing.T) {
	t.Parallel()
	q := startQueue(t)
	ctx := context.Background()
	errs := make(chan error, 16)
	_, boris := q.client(t, errs)
	_, err := boris.Publish(ctx, "changes.boris", []byte("изменение Бориса"), jetstream.WithMsgID("boris-1"))
	require.NoError(t, err, "изменение Бориса не принято")
	closed := make(chan struct{})
	anna, js := q.client(t, errs, nats.ClosedHandler(func(*nats.Conn) { close(closed) }))
	ack, err := js.Publish(ctx, "changes.anna", []byte("изменение Анны"), jetstream.WithMsgID("anna-1"))
	require.NoError(t, err, "клиент не добавил изменение в очередь")
	assert.False(t, ack.Duplicate, "новое изменение отмечено как дубль")
	ack, err = js.Publish(ctx, "changes.anna", []byte("изменение Анны"), jetstream.WithMsgID("anna-1"))
	require.NoError(t, err, "повтор изменения не подтверждён")
	assert.True(t, ack.Duplicate, "повтор изменения не отмечен как дубль")
	for _, rollup := range []string{"all", "sub"} {
		msg := nats.NewMsg("changes.boris")
		msg.Header.Set("Nats-Rollup", rollup)
		_, err = js.PublishMsg(ctx, msg)
		assert.Error(t, err, "очередь приняла свёртку %q", rollup)
	}
	for _, subject := range []string{"changes.>", "_INBOX.>", "_INBOX.adm.>"} {
		_, err = anna.SubscribeSync(subject)
		require.NoError(t, err, "подписка %q не отправлена", subject)
	}
	for _, api := range []string{"STREAM.MSG.GET", "STREAM.MSG.DELETE", "STREAM.PURGE", "STREAM.DELETE", "CONSUMER.CREATE", "DIRECT.GET"} {
		require.NoError(t, anna.Publish("$JS.API."+api+".CHANGES", []byte("{}")), "запрос %q не отправлен", api)
	}
	for _, reply := range []string{"$JS.API.STREAM.PURGE.CHANGES", "$JS.API.STREAM.DELETE.CHANGES"} {
		require.NoError(t, anna.PublishRequest("changes.anna", reply, []byte("подмена")), "подмена %q не отправлена", reply)
	}
	require.NoError(t, anna.Flush(), "очередь не ответила клиенту")
	anna.Close()
	<-closed
	snaps.MatchSnapshot(t, violations(errs))
	assert.Never(t, func() bool {
		stream, err := q.admin.Stream(ctx, "CHANGES")
		if err != nil {
			return true
		}
		msg, err := stream.GetLastMsgForSubject(ctx, "changes.boris")
		return err != nil || string(msg.Data) != "изменение Бориса"
	}, 500*time.Millisecond, 50*time.Millisecond, "изменение Бориса пропало из очереди")
	stream, err := q.admin.Stream(ctx, "CHANGES")
	require.NoError(t, err, "поток CHANGES недоступен")
	assert.Equal(t, uint64(4), stream.CachedInfo().State.Msgs, "в потоке не те изменения")
}

func violations(errs <-chan error) string {
	var got []string
	for len(errs) > 0 {
		got = append(got, (<-errs).Error())
	}
	slices.Sort(got)
	return strings.Join(got, "\n")
}

func TestQueueRefusals(t *testing.T) {
	t.Parallel()
	q := startQueue(t)
	_, err := nats.Connect(q.url, nats.UserInfo(q.user, "wrong-pass"), nats.RootCAs(q.ca))
	assert.ErrorIs(t, err, nats.ErrAuthorization, "очередь пустила клиента с неверным паролем")
	sandboxtest.RefusesPlain(t, q.addr, q.user, "client-pass")
}
