package main

import (
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/nats-io/nats.go/jetstream"
)

//go:embed stream.json
var streamJSON string

func loadStream() (jetstream.StreamConfig, error) {
	var cfg jetstream.StreamConfig
	if err := json.Unmarshal([]byte(streamJSON), &cfg); err != nil {
		return cfg, err
	}
	if len(cfg.Subjects) == 0 {
		return cfg, errors.New("не принимает записи")
	}
	return cfg, nil
}
