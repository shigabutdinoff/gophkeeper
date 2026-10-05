package account

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(snapstest.CleanM{M: m}.Run())
}
