package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const projectDir = "../.."

var serialScripts sync.Mutex

func TestMain(m *testing.M) {
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"gophkeeper": func() { keyring.MockInit(); main() }})
}

func TestScripts(t *testing.T) {
	project, err := filepath.Abs(projectDir)
	require.NoError(t, err, "путь к проекту не получен")
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		UpdateScripts:       snapstest.UpdateScripts(),
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			serialScripts.Lock()
			env.Defer(serialScripts.Unlock)
			env.Vars = append(env.Vars, "PROJECT="+project,
				"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
				"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
				"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
			return nil
		},
	})
}
