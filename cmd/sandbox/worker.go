package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/sync/errgroup"

	"github.com/shigabutdinoff/gophkeeper/internal/access"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
	"github.com/shigabutdinoff/gophkeeper/internal/worker"
)

var workerClient = http.DefaultClient

func runWorker(ctx context.Context, s *server.Server, state, seed string) (*errgroup.Group, error) {
	c, err := parseEnv[serviceConfig]()
	if err != nil {
		return nil, err
	}
	admin, err := sandboxUser(state, adminUser)
	if err != nil {
		return nil, err
	}
	nc, err := nats.Connect("", nats.InProcessServer(s), admin)
	if err != nil {
		return nil, fmt.Errorf("подключение обработчика к очереди песочницы: %w", err)
	}
	js, jerr := jetstream.New(nc)
	stop, err := access.Serve(ctx, nc, seed, "", string(c.Supabase.URL), workerClient, s)
	if err = errors.Join(jerr, err); err != nil {
		nc.Close()
		return nil, err
	}
	var g errgroup.Group
	g.Go(func() error {
		defer nc.Close()
		defer stop()
		return worker.Run(ctx, js, worker.Server{URL: string(c.Supabase.URL), Key: c.ServiceKey, Client: workerClient, Log: log.New(os.Stderr, "", log.LstdFlags),
			Files: storage.New(workerClient, string(c.Supabase.URL), c.Supabase.Region, c.Supabase.Key, c.ServiceKey), Seed: seed})
	})
	return &g, nil
}
