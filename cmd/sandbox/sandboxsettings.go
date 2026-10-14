package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const (
	sandboxHost = "127.0.0.1"
	sandboxName = "sandbox"
)

var (
	sandboxPort  = 54222
	sandboxQueue = "tls://" + net.JoinHostPort(sandboxHost, strconv.Itoa(sandboxPort))
)

func sandboxSettings(state string) ([]byte, error) {
	c, cert, err := backend(state)
	if err != nil {
		return nil, err
	}
	ca, err := os.ReadFile(filepath.Join(state, caFile))
	if err != nil {
		return nil, fmt.Errorf("CA песочницы: %w", err)
	}
	data, err := yaml.Marshal(config.Settings{Server: string(c.Supabase.URL), AppKey: c.Supabase.Key, Region: c.Supabase.Region,
		Queue: sandboxQueue, QueueCA: string(ca), ServerCert: cert})
	if err != nil {
		return nil, fmt.Errorf("разметка настроек песочницы: %w", err)
	}
	return marked(sandboxName, data), nil
}
