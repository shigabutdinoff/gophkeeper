package main

import (
	"net/http"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

var httpClient = http.DefaultClient

func workerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Применять изменения из очереди к серверу",
		Long: "Применяет изменения из очереди к серверу по одному в порядке прихода и отбрасывает неразборчивые.\n" +
			"Пускает в очередь только вошедшие устройства: проверяет токен доступа по ключам сервера.",
		Args: cli.NoArgs,
	}
	var s settings
	flags := addFlags(cmd.Flags(), &s)
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		if err := load(c.Flags(), flags); err != nil {
			return err
		}
		if !strings.HasPrefix(s.Server, "https://") {
			return config.ErrInsecureServer
		}
		nc, err := queue.Connect(s.queue(), nats.MaxReconnects(-1))
		if err != nil {
			return err
		}
		defer nc.Close()
		ctx, stop := signal.NotifyContext(c.Context(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		defer stop()
		return run(ctx, nc, s, c.ErrOrStderr())
	}
	return cmd
}
