package main

import (
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
)

func main() {
	log.SetFlags(0)
	if err := newRoot(os.Stdout, os.Stderr).Execute(); err != nil {
		log.Fatal("Ошибка: ", err)
	}
}

func newRoot(out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:               "gophkeeperd",
		Short:             "Сервер GophKeeper",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.AddCommand(workerCmd(), serverCmd())
	cli.Localize(root)
	return root
}
