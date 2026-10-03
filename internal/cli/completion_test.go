package cli

import (
	"slices"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCompleteRequest(t *testing.T) {
	for _, args := range slices.Concat([][]string{{"-h"}, {"help", "completion"}}, paths()) {
		t.Run(name(args), func(t *testing.T) {
			request := slices.Concat([]string{cobra.ShellCompRequestCmd}, args, []string{""})
			out, errOut, err := execute(request...)
			require.NoError(t, err, "запрос автодополнения завершился ошибкой")
			snaps.MatchSnapshot(t, out, errOut)
		})
	}
}
