package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			err = fmt.Errorf("%w: %s", err, bytes.TrimSpace(exit.Stderr))
		}
		return "", fmt.Errorf("команда %q: %w", "git "+strings.Join(args, " "), err)
	}
	return string(out), nil
}

func tags() ([]string, error) {
	out, err := git("tag", "--list")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}
