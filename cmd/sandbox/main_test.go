package main

import (
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const testPortEnv = "SANDBOX_TEST_PORT"

func TestMain(m *testing.M) {
	port := os.Getenv(testPortEnv)
	if port == "" {
		port = freePort()
		os.Setenv(testPortEnv, port)
	}
	sandboxPort, _ = strconv.Atoi(port)
	sandboxQueue = "tls://" + net.JoinHostPort(sandboxHost, port)
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"sandbox": main})
}

func freePort() string {
	l, err := net.Listen("tcp", sandboxHost+":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}
