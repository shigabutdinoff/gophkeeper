package main

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
)

var formatCases = []struct {
	version string
	count   int
}{
	{"27.0", 41}, {"27.0.1", 44}, {"27.0.0", 41}, {"27.25", 3},
	{"27.1-beta", 7}, {"27.1-beta.1", 7}, {"27.1-beta.2", 12}, {"27.1-beta.26", 1234},
	{"27.1-rc", 22}, {"27.1-rc.2", 23}, {"27.1", 23}, {"28.0-beta", 9},
	{"27.1-alpha.1", 1}, {"27.26", 1}, {"27.1-beta.27", 1}, {"27.1-beta.0", 1},
	{"v27.0", 1}, {"27", 1}, {"", 1}, {"27.01", 1}, {"27.0+meta", 1},
	{"99999999999999999999.0", 1},
}

func TestFormat(t *testing.T) {
	for _, tc := range formatCases {
		t.Run(tc.version, func(t *testing.T) {
			v, err := parse(tc.version)
			if err != nil {
				snaps.MatchSnapshot(t, err.Error())
				return
			}
			snaps.MatchSnapshot(t, format(v, tc.count))
		})
	}
}

func TestBase(t *testing.T) {
	tags := []string{"27.0.1", "27.0.0", "27.0-rc", "26.3", "27.0", "v27.1", "27.1-beta", "27.1", "27.2", "nightly", "99999999999999999999.0"}
	for _, current := range []string{"26.0", "27.0", "27.1", "27.2", "28.0"} {
		t.Run(current, func(t *testing.T) {
			v, err := parse(current)
			require.NoError(t, err, "версия %q не разобрана", current)
			snaps.MatchSnapshot(t, base(tags, v))
		})
	}
	t.Run("none", func(t *testing.T) {
		v, err := parse("27.1")
		require.NoError(t, err, "версия 27.1 не разобрана")
		snaps.MatchSnapshot(t, base(nil, v))
	})
}
