package main

import (
	"errors"

	"github.com/nats-io/nats.go"
)

type queueAccess struct {
	URL           string
	Admin, Client nats.Option
}

func refusesStranger(q queueAccess) error {
	nc, err := nats.Connect(q.URL, q.Client, nats.Timeout(queueTimeout))
	if err == nil {
		nc.Close()
		return errors.New("очередь пускает устройство без входа, включите в ней службу доступа")
	}
	return nil
}
