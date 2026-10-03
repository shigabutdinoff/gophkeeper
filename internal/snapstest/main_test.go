package snapstest

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(CleanM{m}.Run())
}
