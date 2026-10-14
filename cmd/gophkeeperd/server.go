package main

import (
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
	"github.com/shigabutdinoff/gophkeeper/internal/server"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Принимать запросы клиентов по защищённому соединению",
		Long: "Принимает запросы клиентов по защищённому соединению и отказывает запросам без ключа проекта.\n" +
			"Регистрирует пользователей и выполняет вход, учётные записи хранит в базе данных и сам создаёт в ней таблицы.\n" +
			"Соединение без целого запроса закрывает через 30 секунд, при остановке сначала отвечает на начатые запросы.",
		Args: cli.NoArgs,
	}
	var s serverSettings
	flags := addFlags(cmd.Flags(), &s)
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		if err := load(c.Flags(), flags); err != nil {
			return err
		}
		if s.Cert == "" || s.Key == "" {
			return server.ErrNoCert
		}
		l, err := net.Listen("tcp", s.Address)
		if err != nil {
			return fmt.Errorf("адрес %s не открыт: %w", s.Address, err)
		}
		defer l.Close()
		ctx, stop := signal.NotifyContext(c.Context(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		defer stop()
		return server.Serve(ctx, l, server.Settings{Cert: s.Cert, Key: s.Key, AppKey: s.AppKey, ServerKey: s.ServerKey,
			TokenKey: s.TokenKey, Database: s.Database, Log: c.ErrOrStderr()})
	}
	return cmd
}
