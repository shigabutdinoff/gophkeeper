package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/server"
)

var workerClient = http.DefaultClient

func ownServer(state string) (func(context.Context, *natsserver.Server) error, error) {
	c, cert, err := backend(state)
	if cert == "" || err != nil {
		return func(context.Context, *natsserver.Server) error { return nil }, err
	}
	l, err := net.Listen("tcp", ownAddr)
	if err != nil {
		return nil, fmt.Errorf("адрес своего сервера %s не открыт: %w", ownAddr, err)
	}
	return func(ctx context.Context, s *natsserver.Server) error {
		admin, err := sandboxUser(state, adminUser)
		if err != nil {
			return err
		}
		nc, err := nats.Connect("", nats.InProcessServer(s), admin)
		if err != nil {
			return fmt.Errorf("подключение своего сервера к очереди песочницы: %w", err)
		}
		defer nc.Close()
		js, _ := jetstream.New(nc)
		return server.Serve(ctx, l, server.Settings{Cert: filepath.Join(state, certFile), Key: filepath.Join(state, keyFile),
			AppKey: c.Supabase.Key, ServerKey: c.ServiceKey, TokenKey: filepath.Join(state, tokenKeyFile), Database: ownDatabase, Log: os.Stderr,
			Queue: js, FilesLimit: 1 << 30})
	}, nil
}
