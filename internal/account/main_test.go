package account

import (
	"os"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	retryWait, retryMax = time.Millisecond, 5*time.Millisecond
	os.Exit(snapstest.CleanM{M: m}.Run())
}
