package queue

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const local = "outbox."

func outboxRoot() (string, error) {
	dir, err := config.Dir()
	return filepath.Join(dir, "outbox"), err
}

func (o *Outbox) start(ctx context.Context, s config.Settings, user, data string) error {
	opts, err := deviceOptions(s, o.dir, data, o.access)
	if err == nil {
		o.srv, err = server.NewServer(opts)
	}
	if err != nil {
		return err
	}
	o.srv.SetLoggerV2(o.denied, false, false, false)
	o.srv.Start()
	if !o.srv.ReadyForConnections(deadline) {
		return errors.New("сервер очереди не запустился")
	}
	o.nc, err = nats.Connect("", nats.InProcessServer(o.srv), nats.CustomInboxPrefix(nats.InboxPrefix+user))
	if err != nil {
		return err
	}
	o.js, err = jetstream.New(o.nc)
	if err == nil {
		o.box, err = o.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: "OUTBOX", Subjects: []string{local + ">"}, Storage: jetstream.FileStorage})
	}
	return err
}
