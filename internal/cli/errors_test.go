package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrors(t *testing.T) {
	cases := [][]string{{"-x"}, {"-hx"}, {"--help=да"}, {"---x"}}
	for _, path := range paths() {
		cases = append(cases, slices.Concat(path, []string{"nosuch"}), slices.Concat(path, []string{"--nosuch"}))
	}
	for _, args := range cases {
		t.Run(name(args), func(t *testing.T) {
			out, errOut, err := execute(args...)
			require.Error(t, err, "команда %q завершилась без ошибки", args)
			assert.Empty(t, out, "при ошибке напечатан stdout")
			assert.Empty(t, errOut, "при ошибке напечатан stderr")
			snaps.MatchSnapshot(t, err.Error())
		})
	}
}
