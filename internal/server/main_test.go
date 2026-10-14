package server

import (
	"os"
	"testing"
	"time"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	pauses = []time.Duration{300 * time.Millisecond, 600 * time.Millisecond}
	os.Exit(snapstest.CleanM{M: m}.Run())
}
