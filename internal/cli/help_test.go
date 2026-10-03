package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var englishLeftovers = []string{
	"Usage:", "Available Commands", "Flags:", "Global Flags", "help for",
	"Generate the autocompletion", "for more information", "Additional help topics",
	"[flags]", "[command]", "--no-descriptions", "version for",
}

func TestHelp(t *testing.T) {
	for _, path := range paths() {
		t.Run(name(path), func(t *testing.T) {
			out := mustExecute(t, slices.Concat(path, []string{"--help"})...)
			snaps.MatchSnapshot(t, out)
			short := mustExecute(t, slices.Concat(path, []string{"-h"})...)
			assert.Equal(t, out, short, "справка по -h отличается от --help")
			topic := mustExecute(t, slices.Concat([]string{"help"}, path)...)
			assert.Equal(t, out, topic, "справка через help отличается от --help")
			for _, phrase := range englishLeftovers {
				assert.NotContains(t, out, phrase, "в справке осталось %q", phrase)
			}
		})
	}
}

func TestMousetrap(t *testing.T) {
	snaps.MatchSnapshot(t, cobra.MousetrapHelpText)
}
