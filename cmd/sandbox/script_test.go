package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rogpeppe/go-internal/testscript"
	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
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
				"GORACE=atexit_sleep_ms=0", testPortEnv+"="+strconv.Itoa(sandboxPort), "SUPABASE_URL=https://project.supabase.co", "SUPABASE_ANON_KEY=ЗАГЛУШКА КЛЮЧА")
			env.Defer(func() {
				stop(context.Background(), filepath.Join(config, "gophkeeper", sandboxName))
			})
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){"publish": publish},
	})
}

func publish(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("использование: publish <тема>")
	}
	var c config.Settings
	ts.Check(yaml.Unmarshal([]byte(ts.ReadFile(filepath.Join(ts.Getenv("CONFIG"), "gophkeeper", config.File))), &c))
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(c.QueueCA)) {
		ts.Fatalf("CA очереди не разобран")
	}
	nc, err := nats.Connect(c.Queue, nats.UserInfo(c.QueueUser, c.QueuePass), nats.CustomInboxPrefix(c.QueueInbox),
		nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}))
	ts.Check(err)
	defer nc.Close()
	js, err := jetstream.New(nc)
	ts.Check(err)
	ack, err := js.Publish(context.Background(), args[0], []byte("изменение"))
	ts.Check(err)
	_, err = fmt.Fprintln(ts.Stdout(), ack.Sequence)
	ts.Check(err)
}
