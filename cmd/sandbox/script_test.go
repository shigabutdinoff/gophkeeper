package main

import (
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
			home := filepath.Join(env.WorkDir, "home")
			config := filepath.Join(home, ".config")
			env.Vars = append(env.Vars, "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+config, "CONFIG="+config,
				"GORACE=atexit_sleep_ms=0", "SUPABASE_URL=https://project.supabase.co", "SUPABASE_ANON_KEY=ЗАГЛУШКА КЛЮЧА")
			return nil
		},
	})
}
