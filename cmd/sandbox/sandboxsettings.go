package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

const (
	sandboxHost = "127.0.0.1"
	sandboxPort = 54222
	sandboxName = "sandbox"
)

var sandboxQueue = "tls://" + net.JoinHostPort(sandboxHost, strconv.Itoa(sandboxPort))

func sandboxSettings(state string) ([]byte, error) {
	s, err := parseEnv[supabaseConfig]()
	if err != nil {
		return nil, err
	}
	ca, err := os.ReadFile(filepath.Join(state, caFile))
	if err != nil {
		return nil, fmt.Errorf("CA песочницы: %w", err)
	}
	client, err := readPass(state, clientUser)
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(settings{
		Server: s.URL, AppKey: s.Key, Queue: sandboxQueue, QueueUser: clientUser, QueuePass: string(client),
		QueueInbox: queueInbox, QueueCA: string(ca),
	})
	if err != nil {
		return nil, fmt.Errorf("разметка настроек песочницы: %w", err)
	}
	return marked(sandboxName, data), nil
}
