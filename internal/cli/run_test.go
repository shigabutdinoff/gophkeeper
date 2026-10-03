package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestRun(t *testing.T) {
	for _, path := range slices.Concat(paths(), [][]string{{"--version"}, {"-v"}}) {
		t.Run(name(path), func(t *testing.T) {
			snaps.MatchSnapshot(t, mustExecute(t, path...))
		})
	}
}
