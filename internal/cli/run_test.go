package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestRun(t *testing.T) {
	runnable := slices.DeleteFunc(paths(), func(path []string) bool { return slices.Equal(path, []string{"register"}) })
	for _, path := range slices.Concat(runnable, [][]string{{"--version"}, {"-v"}}) {
		t.Run(name(path), func(t *testing.T) {
			snaps.MatchSnapshot(t, mustExecute(t, path...))
		})
	}
}
