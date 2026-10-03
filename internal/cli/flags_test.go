package cli

import (
	"errors"
	"io"
	"regexp"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

var flagArgs = []string{"--name", "-n", "--nosuch", "-x", "-hx", "--help=да", "---x", "--name=x"}

func parseFlag(arg string) error {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringP("name", "n", "", "")
	flags.BoolP("help", "h", false, "")
	if err := flags.Parse([]string{arg}); err != nil {
		return err
	}
	return errors.New(arg)
}

func TestFlagError(t *testing.T) {
	for _, arg := range flagArgs {
		t.Run(arg, func(t *testing.T) {
			snaps.MatchSnapshot(t, flagError(nil, parseFlag(arg)).Error())
		})
	}
}

func FuzzFlagError(f *testing.F) {
	for _, arg := range flagArgs {
		f.Add(arg)
	}
	russian := regexp.MustCompile(`^(неизвестный флаг|флагу|неверное значение|неверный синтаксис флага|неверный флаг)`)
	f.Fuzz(func(t *testing.T, arg string) {
		assert.Regexp(t, russian, flagError(nil, parseFlag(arg)).Error(), "ошибка флага не на русском")
	})
}
