package account

import (
	"testing"
	"time"
)

func retryWait() time.Duration {
	if testing.Testing() {
		return time.Millisecond
	}
	return 200 * time.Millisecond
}
