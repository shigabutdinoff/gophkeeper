package main

import (
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const credsFile = "client.creds"

func saveCloud(c cloudConfig, queue, state string, creds []byte) ([]byte, error) {
	credsPath, err := saveCreds(state, creds)
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(cloudSettings(c, queue, credsPath))
	if err != nil {
		return nil, err
	}
	return marked("cloud", data), nil
}

func saveCreds(state string, creds []byte) (string, error) {
	if err := os.MkdirAll(state, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(state, credsFile)
	return path, writeFile(path, creds)
}

func cloudSettings(c cloudConfig, queue, creds string) config.Settings {
	return config.Settings{
		Server: c.Supabase.URL, AppKey: c.Supabase.Key,
		Queue: queue, QueueCreds: creds, QueueInbox: queueInbox,
	}
}
