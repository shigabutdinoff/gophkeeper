package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func terminal(in io.Reader) (int, error) {
	f, ok := in.(interface{ Fd() uintptr })
	if !ok || !isTerminal(int(f.Fd())) {
		return 0, errors.New("пароль вводится только с клавиатуры")
	}
	return int(f.Fd()), nil
}

func hidden(fd int, errOut io.Writer, label string) (string, error) {
	fmt.Fprint(errOut, label)
	stop := restoreOnInterrupt(fd)
	pass, err := readPassword(fd)
	stop()
	fmt.Fprintln(errOut)
	if err != nil {
		return "", fmt.Errorf("чтение пароля: %w", err)
	}
	return string(pass), nil
}

func askHidden(c *cobra.Command, label string) (string, error) {
	fd, err := terminal(c.InOrStdin())
	if err != nil {
		return "", err
	}
	return hidden(fd, c.ErrOrStderr(), label)
}
