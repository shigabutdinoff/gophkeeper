package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
)

func main() {
	log.SetFlags(0)
	if err := newRoot(os.Stdout, os.Stderr, http.DefaultClient).Execute(); err != nil {
		log.Fatal("Ошибка: ", err)
	}
}

func newRoot(out, errOut io.Writer, client *http.Client) *cobra.Command {
	root := &cobra.Command{
		Use:               "gophkeeperd",
		Short:             "Сервер GophKeeper",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.AddCommand(workerCmd(client), serverCmd())
	cli.Localize(root)
	return root
}
