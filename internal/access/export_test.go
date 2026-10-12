package access

import (
	"testing"
	"time"
)

func SetIdle(t *testing.T, d time.Duration) {
	old := idle
	idle = d
	t.Cleanup(func() { idle = old })
}
