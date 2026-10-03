package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type releaseConfig struct {
	Builds []struct{ Targets []string }
}

func TestReleaseTargets(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(projectDir, ".goreleaser.yaml"))
	require.NoError(t, err, "конфиг goreleaser не прочитан")
	var cfg releaseConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg), "конфиг goreleaser не разобран")
	require.Len(t, cfg.Builds, 1, "в конфиге ожидалась одна сборка")
	snaps.MatchSnapshot(t, strings.Join(cfg.Builds[0].Targets, "\n"))
}
