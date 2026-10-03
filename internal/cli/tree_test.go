package cli

import (
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func paths() [][]string {
	var all [][]string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		all = append(all, strings.Fields(c.CommandPath())[1:])
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(New("", "", "", "", io.Discard, io.Discard))
	return all
}

func name(args []string) string {
	return strings.Join(slices.Concat([]string{"gophkeeper"}, args), " ")
}
