package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/shigabutdinoff/gophkeeper/internal/server"
)

var workerClient = http.DefaultClient

func ownServer(state string) (func(context.Context) error, error) {
	c, cert, err := backend(state)
	if cert == "" || err != nil {
		return func(context.Context) error { return nil }, err
	}
	l, err := net.Listen("tcp", ownAddr)
	if err != nil {
		return nil, fmt.Errorf("адрес своего сервера %s не открыт: %w", ownAddr, err)
	}
	return func(ctx context.Context) error {
		return server.Serve(ctx, l, server.Settings{Cert: filepath.Join(state, certFile), Key: filepath.Join(state, keyFile),
			AppKey: c.Supabase.Key, ServerKey: c.ServiceKey, TokenKey: filepath.Join(state, tokenKeyFile), Database: ownDatabase, Log: os.Stderr})
	}, nil
}
