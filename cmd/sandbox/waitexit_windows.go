package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func waitExit(state string) error {
	data, err := os.ReadFile(filepath.Join(state, pidFile))
	if err != nil {
		return fmt.Errorf("PID очереди песочницы: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("PID очереди песочницы: %w", err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if _, err = p.Wait(); err != nil {
		return fmt.Errorf("ожидание очереди песочницы: %w", err)
	}
	return nil
}
