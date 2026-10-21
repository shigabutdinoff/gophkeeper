package main

import (
	"errors"
	"log"
	"os"

	"charm.land/huh/v2"
	"github.com/shigabutdinoff/gophkeeper/internal/cli"
)

const interruptCode = 130

var (
	buildVersion string
	buildNumber  string
	buildDate    string
	buildCommit  string
)

func main() {
	log.SetFlags(0)
	err := cli.New(buildVersion, buildNumber, buildDate, buildCommit, os.Stdout, os.Stderr).Execute()
	if errors.Is(err, huh.ErrUserAborted) {
		os.Exit(interruptCode)
	}
	if err != nil {
		log.Fatal("Ошибка: ", err)
	}
}
