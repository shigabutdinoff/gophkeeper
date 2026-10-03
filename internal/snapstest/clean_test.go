package snapstest

import (
	"errors"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestExitCode(t *testing.T) {
	var codes []int
	for _, c := range []struct {
		code  int
		dirty bool
		err   error
	}{{0, false, nil}, {2, false, nil}, {0, true, nil}, {0, false, errors.New("сбой")}} {
		codes = append(codes, exitCode(c.code, c.dirty, c.err))
	}
	snaps.MatchSnapshot(t, codes)
}
