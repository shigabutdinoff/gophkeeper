package access_test

import (
	"cmp"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

type liveUser struct {
	Token string `json:"access_token"`
	User  struct {
		ID string `json:"id"`
	} `json:"user"`
}

func signup(t *testing.T, server, app, service, name string) liveUser {
	t.Helper()
	var u liveUser
	c := resty.New().SetBaseURL(server).SetTimeout(30 * time.Second)
	email := name + "+gop6-" + time.Now().Format("20060102150405.000000") + "@example.com"
	resp, err := c.R().SetHeader("apikey", app).SetResult(&u).
		SetBody(map[string]string{"email": email, "password": "live-" + email}).Post("auth/v1/signup")
	require.NoError(t, err, "сервер не принял регистрацию")
	require.NotEmpty(t, u.Token, "нет токена доступа, ответ %d", resp.StatusCode())
	t.Cleanup(func() {
		_, err := c.R().SetHeader("apikey", service).SetAuthToken(service).Delete("auth/v1/admin/users/" + u.User.ID)
		assert.NoError(t, err, "учётная запись %s не удалена", email)
	})
	return u
}

func TestLiveBuckets(t *testing.T) {
	server, app, service := os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_ANON_KEY"), os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	adminCreds, clientCreds, seed := os.Getenv("SYNADIA_ADMIN_CREDS"), os.Getenv("SYNADIA_CLIENT_CREDS"), os.Getenv("GOPHKEEPER_ACCESS_SEED")
	if os.Getenv("GOPHKEEPER_LIVE") == "" || server == "" || app == "" || service == "" || adminCreds == "" || clientCreds == "" || seed == "" {
		t.Skip("нет переменных облачных сервера и очереди")
	}
	const queue = "tls://connect.ngs.global"
	admin, err := nats.Connect(queue, nats.UserCredentials(adminCreds))
	require.NoError(t, err, "администратор не подключился к облачной очереди")
	t.Cleanup(admin.Close)
	creds, err := os.ReadFile(adminCreds)
	require.NoError(t, err, "файл доступа администратора не прочитан")
	token, err := natsjwt.ParseDecoratedJWT(creds)
	require.NoError(t, err, "токен администратора не разобран")
	claims, err := natsjwt.DecodeUserClaims(token)
	require.NoError(t, err, "токен администратора не раскрыт")
	stop, err := access.Serve(t.Context(), admin, seed, cmp.Or(claims.IssuerAccount, claims.Issuer), server, http.DefaultClient,
		logger.NewStdLogger(true, false, false, false, false))
	require.NoError(t, err, "служба доступа не запущена")
	t.Cleanup(func() { _ = stop() })
	js, err := jetstream.New(admin)
	require.NoError(t, err, "JetStream недоступен")
	for _, name := range []string{"anna", "boris", "carol", "dave"} {
		u := signup(t, server, app, service, name)
		t.Cleanup(func() {
			assert.NoError(t, js.DeleteObjectStore(context.Background(), record.Bucket(seed, u.User.ID)), "корзина %s не удалена", name)
		})
		nc, err := nats.Connect(queue, nats.UserCredentials(clientCreds), nats.Token(u.Token), nats.CustomInboxPrefix("_INBOX."+u.User.ID))
		require.NoError(t, err, "устройство %s не подключилось", name)
		t.Cleanup(nc.Close)
		info := putObject(t, nc, openBucket(t, nc, u.User.ID), make([]byte, 1<<20))
		assert.EqualValues(t, 1<<20, info.Size, "объект %s записан не целиком", name)
		status, err := js.ObjectStore(t.Context(), record.Bucket(seed, u.User.ID))
		require.NoError(t, err, "корзина %s не создана", name)
		st, err := status.Status(t.Context())
		require.NoError(t, err, "состояние корзины %s не получено", name)
		t.Logf("корзина %s открыта, занято %d байт", name, st.Size())
	}
}
