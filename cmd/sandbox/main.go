package main

import (
	"io"
	"log"
	"os"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
	"github.com/spf13/cobra"
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
		Use: "init", Short: "Подготовить песочницу в контейнере init", Hidden: true, Args: cli.NoArgs, RunE: sandboxInit,
	})
	cli.Localize(root)
	return root
}
