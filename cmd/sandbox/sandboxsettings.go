package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"go.yaml.in/yaml/v3"
)

const sandboxQueue = "tls://127.0.0.1:54222"

func sandboxSettings(state string, client []byte) ([]byte, error) {
	s, err := env.ParseAs[struct {
		URL string `env:"SUPABASE_URL,notEmpty"`
		Key string `env:"SUPABASE_ANON_KEY,notEmpty"`
	}]()
	if err != nil {
		return nil, envError(err)
	}
	if err := httpsOnly(s.URL); err != nil {
		return nil, err
	}
	ca, err := os.ReadFile(filepath.Join(state, "ca.pem"))
	if err != nil {
		return nil, fmt.Errorf("CA песочницы: %w", err)
	}
	data, err := yaml.Marshal(settings{
		Server: s.URL, AppKey: s.Key, Queue: sandboxQueue, QueueUser: "client", QueuePass: string(client),
		QueueInbox: queueInbox, QueueCA: string(ca),
	})
	return marked("sandbox", data), err
}
