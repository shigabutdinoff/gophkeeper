package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	startTimeout = 30 * time.Second
	startPoll    = 50 * time.Millisecond
)

func start(ctx context.Context, state string, q queueAccess) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("путь к программе песочницы: %w", err)
	}
	logPath := filepath.Join(state, "sandbox.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("журнал песочницы: %w", err)
	}
	defer log.Close()
	cmd := &exec.Cmd{Path: exe, Args: []string{os.Args[0], "serve"}, Stdout: log, Stderr: log, SysProcAttr: detached()}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("запуск очереди песочницы: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	return awaitQueue(ctx, q, exited, logPath)
}
