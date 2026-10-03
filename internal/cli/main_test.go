package cli

import (
	"os"
	"testing"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	os.Exit(snapstest.CleanM{M: m}.Run())
}
