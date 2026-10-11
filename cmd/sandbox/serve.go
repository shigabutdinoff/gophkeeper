package main

import (
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/spf13/cobra"
)

func serve(cmd *cobra.Command, _ []string) error {
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	opts, seed, err := serverOptions(p.state, sandboxPort)
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
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	closeStop, err := listenStop(s, p.state, stop)
	if err != nil {
		return err
	}
	defer closeStop()
	done, err := runWorker(ctx, s, p.state, seed)
	if err != nil {
		return err
	}
	if err = done.Wait(); err != nil {
		s.Errorf("обработчик очереди изменений: %v", err)
		<-ctx.Done()
	}
	return nil
}
