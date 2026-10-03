package main

import (
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	testscript.Main(snapstest.CleanM{M: m}, map[string]func(){"buildnumber": main})
}
