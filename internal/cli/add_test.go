package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

var recordID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`)

func freeQueue(t *testing.T) (*server.Options, config.Settings) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "свободный порт не найден")
	require.NoError(t, l.Close(), "порт не освобождён")
	port := l.Addr().(*net.TCPAddr).Port
	web := httptest.NewTLSServer(http.NotFoundHandler())
	web.Close()
	opts := &server.Options{
		Host: "127.0.0.1", Port: port, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		TLSConfig: &tls.Config{Certificates: web.TLS.Certificates, MinVersion: tls.VersionTLS12},
		Users:     []*server.User{{Username: "client", Password: "client-pass"}},
	}
	return opts, config.Settings{
		Queue: "tls://" + net.JoinHostPort(opts.Host, strconv.Itoa(port)), QueueUser: "client", QueuePass: "client-pass",
		QueueInbox: "_INBOX.pub", QueueCA: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: web.Certificate().Raw})),
	}
}

func startQueue(t *testing.T, opts *server.Options, s config.Settings) jetstream.Stream {
	t.Helper()
	srv := natstest.RunServer(opts)
	t.Cleanup(srv.Shutdown)
	roots := x509Pool(t, s.QueueCA)
	nc, err := nats.Connect(s.Queue, nats.UserInfo(s.QueueUser, s.QueuePass), nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}))
	require.NoError(t, err, "очередь не приняла подключение")
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err, "JetStream недоступен")
	stream, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: "CHANGES", Subjects: []string{"changes.>"}, Storage: jetstream.MemoryStorage})
	require.NoError(t, err, "поток CHANGES не создан")
	return stream
}

func queueSettings(t *testing.T, server string, q config.Settings) {
	t.Helper()
	q.Server, q.AppKey = server, "ключ-приложения"
	writeSettings(t, q)
}

func loggedIn(t *testing.T, q config.Settings) *signupServer {
	t.Helper()
	keyring.MockInit()
	srv := startServer(t, ok)
	queueSettings(t, srv.URL, q)
	_, _, err := executeIn(keyboard(t, true, typedPassword), "login")
	require.NoError(t, err, "вход не прошёл")
	return srv
}

func lastRecord(t *testing.T, stream jetstream.Stream) string {
	t.Helper()
	msg, err := stream.GetLastMsgForSubject(t.Context(), record.Subject)
	require.NoError(t, err, "изменения нет в очереди")
	var body struct{ Record string }
	require.NoError(t, json.Unmarshal(msg.Data, &body), "тело изменения не JSON")
	return body.Record
}

func TestAdd(t *testing.T) {
	opts, q := freeQueue(t)
	stream := startQueue(t, opts, q)
	loggedIn(t, q)
	out, errOut, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna", "--note", "сайт example.com")
	require.NoError(t, err, "запись не принята")
	snaps.MatchSnapshot(t, recordID.ReplaceAllString(out, "<id>"), errOut)
	assert.Contains(t, out, lastRecord(t, stream), "напечатан не тот идентификатор записи")
	info, err := stream.Info(t.Context())
	require.NoError(t, err, "состояние потока не получено")
	assert.Equal(t, uint64(1), info.State.Msgs, "в очереди не одно изменение")
}

func TestAddWithoutNote(t *testing.T) {
	opts, q := freeQueue(t)
	startQueue(t, opts, q)
	loggedIn(t, q)
	_, errOut, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.NoError(t, err, "запись без пометки не принята")
	snaps.MatchSnapshot(t, errOut)
}

func TestAddOffline(t *testing.T) {
	opts, q := freeQueue(t)
	stream := startQueue(t, opts, q)
	srv := loggedIn(t, q)
	srv.Close()
	out, _, err := executeIn(keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.NoError(t, err, "запись без сервера не принята")
	assert.Contains(t, out, lastRecord(t, stream), "напечатан не тот идентификатор записи")
}

func TestAddQueueDown(t *testing.T) {
	_, q := freeQueue(t)
	loggedIn(t, q)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	out, _, err := executeCtx(ctx, keyboard(t, true, "s3cret"), "add", "--login", "anna")
	require.ErrorIs(t, err, queue.ErrUnavailable, "недоступная очередь не распознана")
	assert.Empty(t, out, "при ошибке напечатано подтверждение")
}

func TestAddRefusesBeforeSecrets(t *testing.T) {
	_, q := freeQueue(t)
	plain := q
	plain.Queue = "nats" + q.Queue[len("tls"):]
	cases := map[string]struct {
		settings config.Settings
		args     []string
		terminal bool
	}{
		"insecure":     {plain, []string{"--login", "anna"}, true},
		"no login":     {q, nil, true},
		"not terminal": {q, []string{"--login", "anna"}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			loggedIn(t, c.settings)
			in := keyboard(t, c.terminal, "s3cret")
			setVar(t, &readPassword, func(int) ([]byte, error) { return nil, errors.New("пароль спрошен") })
			out, _, err := executeIn(in, append([]string{"add"}, c.args...)...)
			require.Error(t, err, "запись принята")
			assert.Empty(t, out, "при ошибке напечатано подтверждение")
			snaps.MatchSnapshot(t, err.Error())
		})
	}
}

func x509Pool(t *testing.T, ca string) *x509.CertPool {
	t.Helper()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(ca)), "CA очереди не разобран")
	return roots
}
