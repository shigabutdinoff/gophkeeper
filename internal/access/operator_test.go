package access_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

type operatorQueue struct {
	server          *server.Server
	account, seed   string
	admin, sentinel nats.Option
}

func pair(t *testing.T, create func() (nkeys.KeyPair, error)) (nkeys.KeyPair, string, string) {
	t.Helper()
	kp, err := create()
	require.NoError(t, err, "ключ не создан")
	seed, _ := kp.Seed()
	public, _ := kp.PublicKey()
	return kp, string(seed), public
}

func runOperator(t *testing.T) operatorQueue {
	t.Helper()
	operator, _, operatorKey := pair(t, nkeys.CreateOperator)
	_, _, system := pair(t, nkeys.CreateAccount)
	owner, _, account := pair(t, nkeys.CreateAccount)
	_, seed, signer := pair(t, nkeys.CreateAccount)
	user := func(limit func(*natsjwt.UserClaims)) nats.Option {
		_, userSeed, public := pair(t, nkeys.CreateUser)
		claims := natsjwt.NewUserClaims(public)
		limit(claims)
		token, err := claims.Encode(owner)
		require.NoError(t, err, "пользователь очереди не подписан")
		return nats.UserJWTAndSeed(token, userSeed)
	}
	var adminKey string
	admin := user(func(c *natsjwt.UserClaims) { adminKey = c.Subject })
	sentinel := user(func(c *natsjwt.UserClaims) { c.Pub.Deny.Add(">"); c.Sub.Deny.Add(">") })
	resolver := &server.MemAccResolver{}
	for key, edit := range map[string]func(*natsjwt.AccountClaims){system: func(*natsjwt.AccountClaims) {},
		account: func(c *natsjwt.AccountClaims) {
			c.SigningKeys.Add(signer)
			c.Authorization.AuthUsers.Add(adminKey)
			c.Limits.JetStreamLimits = natsjwt.JetStreamLimits{MemoryStorage: -1, DiskStorage: -1, Streams: -1, Consumer: -1}
		}} {
		claims := natsjwt.NewAccountClaims(key)
		edit(claims)
		token, err := claims.Encode(operator)
		require.NoError(t, err, "учётная запись очереди не подписана")
		require.NoError(t, resolver.Store(key, token), "учётная запись очереди не сохранена")
	}
	trusted := natsjwt.NewOperatorClaims(operatorKey)
	trusted.Issuer = operatorKey
	s := natstest.RunServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		TrustedOperators: []*natsjwt.OperatorClaims{trusted}, SystemAccount: system, AccountResolver: resolver})
	t.Cleanup(s.Shutdown)
	return operatorQueue{server: s, account: account, seed: seed, admin: admin, sentinel: sentinel}
}

func TestOperatorMode(t *testing.T) {
	q, o := queuetest.New(t), runOperator(t)
	service, err := nats.Connect(o.server.ClientURL(), o.admin)
	require.NoError(t, err, "служба не подключилась к очереди")
	t.Cleanup(service.Close)
	stop, err := access.Serve(t.Context(), service, o.seed, o.account, q.Supabase, http.DefaultClient, o.server)
	require.NoError(t, err, "служба доступа не запущена")
	t.Cleanup(func() { stop() })

	_, err = nats.Connect(o.server.ClientURL(), o.sentinel)
	assert.ErrorIs(t, err, nats.ErrAuthorization, "очередь пустила устройство без входа")

	errs := make(chan string, 8)
	nc, err := nats.Connect(o.server.ClientURL(), o.sentinel, q.As(t, anna),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { errs <- err.Error() }))
	require.NoError(t, err, "вошедшее устройство не подключилось")
	t.Cleanup(nc.Close)
	got, err := service.SubscribeSync("changes.>")
	require.NoError(t, err, "служба не подписалась на изменения")
	require.NoError(t, service.Flush(), "очередь не ответила службе")
	require.NoError(t, nc.Publish(record.Topic(boris), []byte("чужое")), "чужое изменение не отправлено")
	require.NoError(t, nc.Publish(record.Topic(anna), []byte("своё")), "своё изменение не отправлено")
	_, err = nc.SubscribeSync("changes.>")
	require.NoError(t, err, "подписка на изменения не отправлена")
	require.NoError(t, nc.Flush(), "очередь не ответила устройству")
	msg, err := got.NextMsg(2 * time.Second)
	require.NoError(t, err, "своё изменение не дошло")
	assert.Equal(t, record.Topic(anna), msg.Subject, "дошло чужое изменение")
	snaps.MatchSnapshot(t, []string{<-errs, <-errs})
	assert.EqualValues(t, 19, putObject(t, nc, openBucket(t, nc, anna), []byte("шифр файла")).Size, "файл в режиме оператора записан не целиком")
}

func TestServeBadAccount(t *testing.T) {
	q, o := queuetest.New(t), runOperator(t)
	service, err := nats.Connect(o.server.ClientURL(), o.admin)
	require.NoError(t, err, "служба не подключилась к очереди")
	t.Cleanup(service.Close)
	_, err = access.Serve(t.Context(), service, o.seed, o.seed, q.Supabase, http.DefaultClient, o.server)
	require.Error(t, err, "служба приняла закрытый ключ вместо учётной записи")
	assert.NotContains(t, err.Error(), o.seed, "в ошибке закрытый ключ")
	snaps.MatchSnapshot(t, err.Error())
}
