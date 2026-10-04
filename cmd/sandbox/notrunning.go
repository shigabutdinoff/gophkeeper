package main

import (
	"errors"
	"net"

	"github.com/nats-io/nats.go"
)

func notRunning(err error) bool {
	op, ok := errors.AsType[*net.OpError](err)
	return errors.Is(err, nats.ErrNoServers) || ok && op.Op == "dial" && !op.Timeout()
}
