package main

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/spf13/cobra"
)

func serve(*cobra.Command, []string) error {
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	opts, err := serverOptions(p.state, sandboxPort)
	if err != nil {
		return err
	}
	s, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("создание очереди песочницы: %w", err)
	}
	s.ConfigureLogger()
	s.Start()
	defer s.Shutdown()
	if !s.ReadyForConnections(queueTimeout) {
		return errors.New("очередь песочницы не запустилась")
	}
	closeStop, err := listenStop(s, p.state)
	if err != nil {
		return err
	}
	defer closeStop()
	s.WaitForShutdown()
	return nil
}
