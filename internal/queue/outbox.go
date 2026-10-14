package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rogpeppe/go-internal/lockedfile"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Outbox хранит изменения на устройстве до подтверждения очереди изменений.
type Outbox struct {
	srv    *server.Server
	nc     *nats.Conn
	js     jetstream.JetStream
	box    jetstream.Stream
	denied *denial
	unlock func()
	dir    string
	access string
}

// Open открывает очередь устройства пользователя user и ждёт, пока её не
// освободит другая команда. Ключ data шифрует её на диске, токен access
// пускает к очереди изменений из настроек s.
func Open(ctx context.Context, s config.Settings, user, data, access string) (*Outbox, error) {
	root, err := outboxRoot()
	if err == nil {
		err = os.MkdirAll(root, 0o700)
	}
	var unlock func()
	if err == nil {
		unlock, err = lockedfile.MutexAt(filepath.Join(root, user+".lock")).Lock()
	}
	if err != nil {
		return nil, fmt.Errorf("очередь устройства: %w", err)
	}
	o := &Outbox{denied: &denial{}, unlock: unlock, dir: filepath.Join(root, user), access: access}
	if err = o.start(context.WithoutCancel(ctx), s, user, data); err != nil {
		o.Close()
		return nil, fmt.Errorf("очередь устройства: %w", err)
	}
	return o, nil
}
