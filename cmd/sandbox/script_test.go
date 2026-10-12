package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

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
				"GORACE=atexit_sleep_ms=0", testPortEnv+"="+strconv.Itoa(sandboxPort), "SUPABASE_URL=https://127.0.0.1:9", "SUPABASE_ANON_KEY=ЗАГЛУШКА КЛЮЧА", "SUPABASE_SERVICE_ROLE_KEY=ЗАГЛУШКА КЛЮЧА АДМИНИСТРАТОРА", "SUPABASE_REGION=eu-north-1")
			env.Defer(func() {
				stop(context.Background(), filepath.Join(config, "gophkeeper", sandboxName))
			})
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){"publish": publish, "stranger": stranger},
	})
}

func dial(ts *testscript.TestScript, opts ...nats.Option) (*nats.Conn, error) {
	var c config.Settings
	dir := filepath.Join(ts.Getenv("CONFIG"), "gophkeeper")
	ts.Check(yaml.Unmarshal([]byte(ts.ReadFile(filepath.Join(dir, config.File))), &c))
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(c.QueueCA)) {
		ts.Fatalf("CA очереди не разобран")
	}
	pass := ts.ReadFile(filepath.Join(dir, sandboxName, adminUser))
	return nats.Connect(c.Queue, append([]nats.Option{nats.UserInfo(adminUser, pass),
		nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})}, opts...)...)
}

func publish(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("использование: publish <тема>")
	}
	nc, err := dial(ts)
	ts.Check(err)
	defer nc.Close()
	js, err := jetstream.New(nc)
	ts.Check(err)
	ack, err := js.Publish(context.Background(), args[0], []byte("изменение"))
	ts.Check(err)
	_, err = fmt.Fprintln(ts.Stdout(), ack.Sequence)
	ts.Check(err)
}

func stranger(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("использование: stranger")
	}
	var err error
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline) && !errors.Is(err, nats.ErrAuthorization); {
		var nc *nats.Conn
		if nc, err = dial(ts, nats.UserInfo("", ""), nats.Token("s3cret")); err == nil {
			nc.Close()
			ts.Fatalf("очередь пустила устройство без входа")
		}
	}
	if !errors.Is(err, nats.ErrAuthorization) {
		ts.Fatalf("очередь не отказала устройству без входа: %v", err)
	}
}
