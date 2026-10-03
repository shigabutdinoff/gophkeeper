//go:build e2e

package e2e

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/shigabutdinoff/gophkeeper/cmd/sandbox/internal/sandboxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkPorts(t *testing.T) {
	out, err := exec.Command("docker", "compose", "-p", sandboxtest.Project, "ps", "--format",
		"{{range .Publishers}}{{if .PublishedPort}}{{.URL}}:{{.PublishedPort}} {{end}}{{end}}").Output()
	require.NoError(t, err, "службы песочницы не прочитаны")
	assert.Equal(t, []string{"127.0.0.1:54222"}, strings.Fields(string(out)), "опубликованы не те порты")
}

func queueConn(t *testing.T, c map[string]string, user, password, inbox string) (*nats.Conn, jetstream.JetStream) {
	return sandboxtest.Connect(t, c["queue"], c["queue_ca_file"], nil,
		nats.UserInfo(user, password), nats.CustomInboxPrefix(inbox), nats.NoReconnect())
}

func clientConn(t *testing.T, c map[string]string) (*nats.Conn, jetstream.JetStream) {
	return queueConn(t, c, c["queue_user"], c["queue_password"], c["queue_inbox"])
}

func checkQueue(t *testing.T, c map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	boris, js := clientConn(t, c)
	_, err := js.Publish(ctx, "changes.boris", []byte("изменение Бориса"))
	boris.Close()
	require.NoError(t, err, "очередь не приняла изменение Бориса")
	anna, js := clientConn(t, c)
	_, err = js.Publish(ctx, "changes.anna", []byte("изменение Анны"))
	require.NoError(t, err, "очередь не приняла изменение Анны")
	require.NoError(t, anna.Publish("$JS.API.STREAM.MSG.GET.CHANGES", []byte(`{"last_by_subj":"changes.boris"}`)), "запрос чужого изменения не отправлен")
	require.NoError(t, anna.Flush(), "очередь не ответила клиенту")
	assert.ErrorIs(t, anna.LastError(), nats.ErrPermissionViolation, "очередь не отказала клиенту в чтении")
	sandboxtest.RefusesPlain(t, strings.TrimPrefix(c["queue"], "tls://"), c["queue_user"], c["queue_password"])
}

func lastChange(t *testing.T, c, secrets map[string]string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, js := queueConn(t, c, "admin", secrets["admin"], "_INBOX.adm")
	stream, err := js.Stream(ctx, "CHANGES")
	require.NoError(t, err, "поток CHANGES недоступен")
	msg, err := stream.GetLastMsgForSubject(ctx, "changes.boris")
	require.NoError(t, err, "изменения Бориса нет в потоке")
	return string(msg.Data)
}
