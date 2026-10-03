package main

import (
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/shigabutdinoff/gophkeeper/cmd/sandbox/internal/sandboxtest"
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
			home := filepath.Join(env.WorkDir, "home")
			env.Vars = append(slices.Concat(env.Vars, sandboxtest.HomeEnv(home)), "CONFIG="+sandboxtest.ConfigDir(home),
				"GORACE=atexit_sleep_ms=0", "SUPABASE_URL=https://project.supabase.co", "SUPABASE_ANON_KEY=ЗАГЛУШКА КЛЮЧА")
			return nil
		},
	})
}
