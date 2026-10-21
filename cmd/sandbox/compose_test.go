package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestComposeFile(t *testing.T) {
	data, err := os.ReadFile("../../compose.yaml")
	require.NoError(t, err, "compose.yaml не прочитан")
	var c struct {
		Services map[string]struct {
			Image       string
			Ports       []string
			Healthcheck struct{ Test, Interval string }
		}
	}
	require.NoError(t, yaml.Unmarshal(data, &c), "compose.yaml не разобран")
	require.Len(t, c.Services, 1, "в compose.yaml не одна служба")
	postgres := c.Services["postgres"]
	assert.True(t, strings.HasPrefix(postgres.Image, "postgres:"), "служба не Postgres")
	assert.Contains(t, postgres.Healthcheck.Test, "nc -z ", "готовность проверяется не по TCP")
	assert.Equal(t, "1s", postgres.Healthcheck.Interval, "готовность проверяется не раз в секунду")
	require.Len(t, postgres.Ports, 1, "у базы не один порт")
	assert.True(t, strings.HasPrefix(postgres.Ports[0], "127.0.0.1:"), "порт базы открыт не только на 127.0.0.1")
}
