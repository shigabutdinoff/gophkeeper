package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func flagError(_ *cobra.Command, err error) error {
	if e, ok := errors.AsType[*pflag.NotExistError](err); ok {
		return fmt.Errorf("неизвестный флаг %q", flagName(e.GetSpecifiedName(), e.GetSpecifiedShortnames()))
	}
	if e, ok := errors.AsType[*pflag.ValueRequiredError](err); ok {
		return fmt.Errorf("флагу %q нужно значение", flagName(e.GetSpecifiedName(), e.GetSpecifiedShortnames()))
	}
	if e, ok := errors.AsType[*pflag.InvalidValueError](err); ok {
		return fmt.Errorf("неверное значение %q флага %q", e.GetValue(), "--"+e.GetFlag().Name)
	}
	if e, ok := errors.AsType[*pflag.InvalidSyntaxError](err); ok {
		return fmt.Errorf("неверный синтаксис флага %q", e.GetSpecifiedFlag())
	}
	return errors.New("неверный флаг")
}

func flagName(name, shorthands string) string {
	if shorthands != "" {
		return "-" + name
	}
	return "--" + name
}
