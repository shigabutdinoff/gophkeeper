package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

var serialScripts sync.Mutex

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		UpdateScripts:       snapstest.UpdateScripts(),
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			serialScripts.Lock()
			env.Defer(serialScripts.Unlock)
			env.Vars = append(env.Vars,
				"GIT_CEILING_DIRECTORIES="+filepath.Dir(env.WorkDir),
				"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
				"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
				"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
			return nil
		},
	})
}
