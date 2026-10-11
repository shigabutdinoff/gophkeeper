package main

import (
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

var serialScripts sync.Mutex

func TestMain(m *testing.M) {
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"gophkeeperd": main})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		UpdateScripts:       snapstest.UpdateScripts(),
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			serialScripts.Lock()
			env.Defer(serialScripts.Unlock)
			env.Vars = append(env.Vars, "GORACE=atexit_sleep_ms=0")
			return nil
		},
	})
}
