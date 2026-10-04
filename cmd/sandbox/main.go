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
		Use: "serve", Short: "Обслуживать очередь песочницы", Hidden: true, Args: cli.NoArgs, RunE: serve,
	})
	cli.Localize(root)
	return root
}
