package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/nats-io/nats-server/v2/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
	"github.com/shigabutdinoff/gophkeeper/internal/worker"
)

func run(ctx context.Context, nc *nats.Conn, s settings, errOut io.Writer, client *http.Client) error {
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	stop, err := access.Serve(ctx, nc, s.AccessSeed, s.QueueAccount, s.Server, client, logger.NewStdLogger(true, false, false, false, false))
	if err != nil {
		return err
	}
	defer func() { _ = stop() }()
	return worker.Run(ctx, js, worker.Server{URL: s.Server, Key: s.ServerKey, Client: client,
		Log: log.New(errOut, "", log.LstdFlags), Seed: s.AccessSeed,
		Files: storage.New(client, s.Server, s.Region, s.AppKey, s.ServerKey)})
}
