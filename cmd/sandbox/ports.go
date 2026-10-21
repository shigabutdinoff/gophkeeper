package main

import (
	"cmp"
	"net"
	"os"
	"strconv"
)

const (
	portEnv     = "GOPHKEEPER_SANDBOX_PORT"
	leafPortEnv = "GOPHKEEPER_SANDBOX_LEAF_PORT"
	ownPortEnv  = "GOPHKEEPER_SANDBOX_OWN_PORT"
	ownDBEnv    = "GOPHKEEPER_SANDBOX_DATABASE"
)

func envPort(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return n
	}
	return def
}

func sandboxPort() int {
	return envPort(portEnv, 54222)
}

func leafPort() int {
	return envPort(leafPortEnv, 54223)
}

func sandboxQueue() string {
	return "tls://" + net.JoinHostPort(sandboxHost, strconv.Itoa(sandboxPort()))
}

func ownAddr() string {
	return net.JoinHostPort(sandboxHost, strconv.Itoa(envPort(ownPortEnv, 54443)))
}

func ownDatabase() string {
	return cmp.Or(os.Getenv(ownDBEnv), "postgres://postgres:gophkeeper-sandbox@"+sandboxHost+":54320/postgres?sslmode=disable")
}
