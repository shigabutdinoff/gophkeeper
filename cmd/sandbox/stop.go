package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/nats-io/nats.go"
)

func stop(ctx context.Context, state string) (bool, error) {
	admin, err := sandboxUser(state, adminUser)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	nc, err := nats.Connect(sandboxQueue, admin, nats.NoReconnect())
	if notRunning(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("подключение к очереди песочницы: %w", err)
	}
	defer nc.Close()
	closed := nc.StatusChanged(nats.CLOSED)
	ctx, cancel := context.WithTimeout(ctx, queueTimeout)
	defer cancel()
	if _, err = nc.RequestWithContext(ctx, stopSubject, nil); err != nil {
		return false, fmt.Errorf("остановка очереди песочницы: %w", err)
	}
	select {
	case <-closed:
		return true, waitExit(state)
	case <-ctx.Done():
		return true, fmt.Errorf("очередь песочницы не остановилась: %w", ctx.Err())
	}
}
