package main

import (
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

var serialScripts sync.Mutex

func TestMain(m *testing.M) {
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"gophkeeperd": main})
}

func TestScripts(t *testing.T) {
	q := queuetest.New(t)
	q.Run(t, false)
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		UpdateScripts:       snapstest.UpdateScripts(),
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			serialScripts.Lock()
			env.Defer(serialScripts.Unlock)
			env.Vars = append(env.Vars, "GORACE=atexit_sleep_ms=0", "TEST_QUEUE="+q.Admin.Queue, "TEST_QUEUE_CA="+q.Admin.QueueCA,
				"TEST_QUEUE_USER="+q.Admin.QueueUser, "TEST_QUEUE_PASSWORD="+q.Admin.QueuePass)
			return nil
		},
	})
}
