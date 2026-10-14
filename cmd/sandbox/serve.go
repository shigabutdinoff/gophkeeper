package main

import (
	"errors"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func serve(cmd *cobra.Command, _ []string) error {
	p, err := paths(sandboxName)
	if err != nil {
		return err
	}
	own, err := ownServer(p.state)
	if err != nil {
		return err
	}
	s, seed, err := launchQueue(p.state)
	if err != nil {
		return err
	}
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
	var served errgroup.Group
	served.Go(func() error { return own(ctx, s) })
	done, err := runWorker(ctx, s, p.state, seed)
	if err != nil {
		return err
	}
	if err = done.Wait(); err != nil {
		s.Errorf("обработчик очереди изменений: %v", err)
		<-ctx.Done()
	}
	return served.Wait()
}
