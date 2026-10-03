package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const queueTimeout = 10 * time.Second

func prepareQueue(ctx context.Context, q queueAccess) error {
	cfg, err := loadStream()
	if err != nil {
		return fmt.Errorf("поток изменений: %w", err)
	}
	nc, err := nats.Connect(q.URL, q.Admin)
	if err != nil {
		return fmt.Errorf("подключение к очереди: %w", err)
	}
	defer nc.Close()
	if err := ensureStream(ctx, nc, cfg); err != nil {
		return fmt.Errorf("поток изменений: %w", err)
	}
	return checkClient(q.URL, cfg, q.Client)
}

func ensureStream(ctx context.Context, nc *nats.Conn, cfg jetstream.StreamConfig) error {
	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(queueTimeout))
	if err != nil {
		return err
	}
	_, err = js.CreateStream(ctx, cfg)
	if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		return nil
	}
	return err
}
