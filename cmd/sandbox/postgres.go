package main

import (
	"context"
	"fmt"
	"os/exec"
)

func compose(ctx context.Context, args ...string) error {
	args = append([]string{"compose", "-p", "gophkeeper-sandbox", "-f", "compose.yaml"}, args...)
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose не выполнен, проверьте, что Docker запущен и команда вызвана из корня репозитория: %w\n%s", err, out)
	}
	return nil
}

func enter(ctx context.Context, state string, own bool) error {
	err := checkMode(state, own)
	if err != nil || !own {
		return err
	}
	if err = compose(ctx, "up", "--detach", "--wait"); err != nil {
		_ = compose(ctx, "down")
	}
	return err
}
