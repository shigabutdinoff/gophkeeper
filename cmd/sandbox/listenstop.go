package main

import (
	"fmt"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func listenStop(s *server.Server, state string, stop func()) (func(), error) {
	admin, err := sandboxUser(state, adminUser)
	if err != nil {
		return nil, err
	}
	nc, err := nats.Connect("", nats.InProcessServer(s), admin)
	if err != nil {
		return nil, fmt.Errorf("подключение к очереди песочницы: %w", err)
	}
	_, err = nc.Subscribe(stopSubject, func(m *nats.Msg) {
		if m.Reply == "" {
			return
		}
		s.Noticef("получена команда %s", stopSubject)
		if rerr := m.Respond(nil); rerr != nil {
			s.Errorf("ответ на %s: %v", stopSubject, rerr)
		}
		if ferr := nc.Flush(); ferr != nil {
			s.Errorf("отправка ответа на %s: %v", stopSubject, ferr)
		}
		stop()
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("подписка на %s: %w", stopSubject, err)
	}
	return nc.Close, nil
}
