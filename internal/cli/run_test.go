package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestRun(t *testing.T) {
	runnable := slices.DeleteFunc(paths(), func(path []string) bool {
		return len(path) == 1 && slices.Contains([]string{"register", "login", "logout", "add"}, path[0])
	})
	for _, path := range slices.Concat(runnable, [][]string{{"--version"}, {"-v"}}) {
		t.Run(name(path), func(t *testing.T) {
			snaps.MatchSnapshot(t, mustExecute(t, path...))
		})
	}
}
