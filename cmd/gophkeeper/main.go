package main

import (
	"log"
	"os"

	"github.com/shigabutdinoff/gophkeeper/internal/cli"
)

var (
	buildVersion string
	buildNumber  string
	buildDate    string
	buildCommit  string
)

func main() {
	log.SetFlags(0)
	if err := cli.New(buildVersion, buildNumber, buildDate, buildCommit, os.Stdout, os.Stderr).Execute(); err != nil {
		log.Fatal("Ошибка: ", err)
	}
}
