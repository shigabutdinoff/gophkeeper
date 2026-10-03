package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func commits(tag string) (int, error) {
	shallow, err := git("rev-parse", "--is-shallow-repository")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(shallow) == "true" {
		return 0, errors.New("история репозитория неполная, номер сборки не посчитать")
	}
	args := []string{"rev-list", "--count", "HEAD"}
	if tag != "" {
		args = append(args, "^"+tag)
	}
	out, err := git(append(args, "--")...)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("число коммитов %q: %w", out, err)
	}
	return n, nil
}
