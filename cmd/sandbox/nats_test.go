package main

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const (
	anna  = "0199b5e4-0000-7000-8000-00000000a11a"
	boris = "0199b5e4-0000-7000-8000-0000000b0715"
)

type queue struct {
	url, addr, ca, pass, state string
	server                     *server.Server
	admin                      jetstream.JetStream
	tokens                     *queuetest.Queue
}

func startQueue(t *testing.T) queue {
	state := t.TempDir()
	require.NoError(t, sandboxSecrets(state), "секреты песочницы не созданы")
	pass, err := readPass(state, adminUser)
	require.NoError(t, err, "пароль администратора не прочитан")
	opts, seed, err := serverOptions(state, server.RANDOM_PORT, server.RANDOM_PORT)
	require.NoError(t, err, "настройки очереди песочницы не собраны")
	opts.NoSigs = true
	s := natstest.RunServer(opts)
	t.Cleanup(s.Shutdown)
	q := queue{url: s.ClientURL(), addr: s.Addr().String(), ca: filepath.Join(state, caFile), pass: string(pass), state: state,
		server: s, tokens: queuetest.New(t)}
	admin, err := sandboxUser(state, adminUser)
	require.NoError(t, err, "пароль администратора не прочитан")
	var nc *nats.Conn
	nc, q.admin = connect(t, q.url, q.ca, nil, admin, nats.CustomInboxPrefix("_INBOX.adm"))
	stop, err := access.Serve(t.Context(), nc, seed, "", q.tokens.Supabase, http.DefaultClient, s)
	require.NoError(t, err, "служба доступа не запущена")
	t.Cleanup(func() { stop() })
	stream, err := loadStream()
	require.NoError(t, err, "поток не разбирается")
	_, err = q.admin.CreateStream(context.Background(), stream)
	require.NoError(t, err, "поток CHANGES не создан")
	return q
}

func (q queue) client(t *testing.T, owner string, errs chan<- error, opts ...nats.Option) (*nats.Conn, jetstream.JetStream) {
	return connect(t, q.url, q.ca, errs, append(opts, q.tokens.As(t, owner))...)
}

func TestQueueClient(t *testing.T) {
	t.Parallel()
	q := startQueue(t)
	ctx := context.Background()
	errs := make(chan error, 16)
	own, foreign := record.Topic(anna), record.Topic(boris)
	_, other := q.client(t, boris, errs)
	_, err := other.Publish(ctx, foreign, []byte("изменение Бориса"), jetstream.WithMsgID("boris-1"))
	require.NoError(t, err, "изменение Бориса не принято")
	closed := make(chan struct{})
	device, js := q.client(t, anna, errs, nats.ClosedHandler(func(*nats.Conn) { close(closed) }))
	ack, err := js.Publish(ctx, own, []byte("изменение Анны"), jetstream.WithMsgID("anna-1"))
	require.NoError(t, err, "клиент не добавил изменение в очередь")
	assert.False(t, ack.Duplicate, "новое изменение отмечено как дубль")
	ack, err = js.Publish(ctx, own, []byte("изменение Анны"), jetstream.WithMsgID("anna-1"))
	require.NoError(t, err, "повтор изменения не подтверждён")
	assert.True(t, ack.Duplicate, "повтор изменения не отмечен как дубль")
	for _, rollup := range []string{"all", "sub"} {
		msg := nats.NewMsg(own)
		msg.Header.Set("Nats-Rollup", rollup)
		_, err = js.PublishMsg(ctx, msg)
		assert.Error(t, err, "очередь приняла свёртку %q", rollup)
	}
	for _, subject := range []string{"changes.>", own, "_INBOX.>", "_INBOX.adm.>", "_INBOX." + boris + ".>", "$SYS.REQ.USER.AUTH"} {
		_, err = device.SubscribeSync(subject)
		require.NoError(t, err, "подписка %q не отправлена", subject)
	}
	for _, subject := range []string{foreign, record.Subject, "changes.files." + anna, "$SYS.REQ.USER.AUTH"} {
		require.NoError(t, device.Publish(subject, []byte("чужое")), "публикация в %q не отправлена", subject)
	}
	for _, api := range []string{"STREAM.MSG.GET", "STREAM.MSG.DELETE", "STREAM.PURGE", "STREAM.DELETE", "CONSUMER.CREATE", "DIRECT.GET"} {
		require.NoError(t, device.Publish("$JS.API."+api+".CHANGES", []byte("{}")), "запрос %q не отправлен", api)
	}
	for _, reply := range []string{"$JS.API.STREAM.PURGE.CHANGES", "$JS.API.STREAM.DELETE.CHANGES"} {
		require.NoError(t, device.PublishRequest(own, reply, []byte("подмена")), "подмена %q не отправлена", reply)
	}
	require.NoError(t, device.Flush(), "очередь не ответила клиенту")
	device.Close()
	<-closed
	snaps.MatchSnapshot(t, violations(errs))
	assert.Never(t, func() bool {
		stream, serr := q.admin.Stream(ctx, "CHANGES")
		if serr != nil {
			return true
		}
		msg, merr := stream.GetLastMsgForSubject(ctx, foreign)
		return merr != nil || string(msg.Data) != "изменение Бориса"
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
	for name, opt := range map[string]nats.Option{
		"без входа": nats.Name("посторонний"),
		"прежний пользователь client":    nats.UserInfo("client", q.pass),
		"неверный пароль администратора": nats.UserInfo(adminUser, "wrong-pass"),
		"истёкший токен":                 nats.Token(q.tokens.Token(t, anna, -time.Minute)),
	} {
		_, err := nats.Connect(q.url, opt, nats.RootCAs(q.ca))
		assert.ErrorIs(t, err, nats.ErrAuthorization, "очередь пустила: %s", name)
	}
	refusesPlain(t, q.addr, q.pass)
}
