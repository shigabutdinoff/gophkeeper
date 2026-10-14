package main

import (
	"context"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const (
	testPortEnv    = "SANDBOX_TEST_PORT"
	testOwnPortEnv = "SANDBOX_TEST_OWN_PORT"
	testLeafEnv    = "SANDBOX_TEST_LEAF_PORT"
	testDBEnv      = "GOPHKEEPER_TEST_DATABASE"
	testOwnDBEnv   = "SANDBOX_TEST_OWN_DATABASE"
	testSchema     = "sandbox_test"
)

func TestMain(m *testing.M) {
	port := freePort(testPortEnv)
	sandboxPort, _ = strconv.Atoi(port)
	sandboxQueue = "tls://" + net.JoinHostPort(sandboxHost, port)
	sandboxLeafPort, _ = strconv.Atoi(freePort(testLeafEnv))
	ownAddr = net.JoinHostPort(sandboxHost, freePort(testOwnPortEnv))
	ownDatabase = os.Getenv(testOwnDBEnv)
	if ownDatabase == "" {
		ownDatabase = testDatabase()
		os.Setenv(testOwnDBEnv, ownDatabase)
	}
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"sandbox": main, "docker": fakeDocker})
}

func testDatabase() string {
	base := os.Getenv(testDBEnv)
	if base == "" {
		return "postgres://postgres@" + sandboxHost + ":9/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), base)
	if err != nil {
		panic(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS "+testSchema); err != nil {
		panic(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Set("search_path", testSchema)
	u.RawQuery = q.Encode()
	return u.String()
}

func freePort(env string) string {
	if port := os.Getenv(env); port != "" {
		return port
	}
	l, err := net.Listen("tcp", sandboxHost+":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	os.Setenv(env, port)
	return port
}
