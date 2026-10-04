package main

import (
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
)

const displayName = "go run ./cmd/sandbox"

func main() {
	log.SetFlags(0)
	if err := newRoot(os.Stdout, os.Stderr).Execute(); err != nil {
		log.Fatal("Ошибка: ", err)
	}
}

func newRoot(out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:               "sandbox",
		Short:             "Настройки клиента GophKeeper для проверки",
		Annotations:       map[string]string{cobra.CommandDisplayNameAnnotation: displayName},
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.AddCommand(&cobra.Command{
		Use: "cloud", Short: "Подключить клиент к облачным Supabase и очереди изменений", Args: cli.NoArgs, RunE: cloud,
	}, &cobra.Command{
		Use: "up", Short: "Запустить локальную песочницу в фоне и записать настройки клиента", Args: cli.NoArgs, RunE: up,
	}, &cobra.Command{
		Use: "down", Short: "Остановить локальную песочницу, сохранив её данные", Args: cli.NoArgs, RunE: down,
	}, &cobra.Command{
		Use: "reset", Short: "Остановить локальную песочницу и удалить её данные и настройки", Args: cli.NoArgs, RunE: reset,
	}, &cobra.Command{
		Use: "serve", Short: "Обслуживать очередь песочницы", Hidden: true, Args: cli.NoArgs, RunE: serve,
	})
	cli.Localize(root)
	return root
}
