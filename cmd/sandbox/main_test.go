package main

import (
	"context"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/shigabutdinoff/gophkeeper/internal/queuetest"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

const (
	testDBEnv  = "GOPHKEEPER_TEST_DATABASE"
	testSchema = "sandbox_test"
)

func TestMain(m *testing.M) {
	if err := testEnv(); err != nil {
		log.Fatal("окружение тестов песочницы не готово: ", err)
	}
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"sandbox": main, "docker": func() {
		if fakeDocker() != 0 {
			log.SetOutput(io.Discard)
			log.Fatal()
		}
	}})
}

func testEnv() error {
	for _, env := range []string{portEnv, leafPortEnv, ownPortEnv} {
		if err := freePort(env); err != nil {
			return err
		}
	}
	if os.Getenv(ownDBEnv) != "" {
		return nil
	}
	db, err := testDatabase()
	if err != nil {
		return err
	}
	return os.Setenv(ownDBEnv, db)
}

func testDatabase() (string, error) {
	base := os.Getenv(testDBEnv)
	if base == "" {
		return "postgres://postgres@" + sandboxHost + ":9/postgres?sslmode=disable", nil
	}
	conn, err := pgx.Connect(context.Background(), base)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err = conn.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS "+testSchema); err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("search_path", testSchema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func freePort(env string) error {
	if os.Getenv(env) != "" {
		return nil
	}
	port, err := queuetest.FreePort()
	if err != nil {
		return err
	}
	return os.Setenv(env, strconv.Itoa(port))
}
