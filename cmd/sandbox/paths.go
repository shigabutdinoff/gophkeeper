package main

import (
	"path/filepath"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

type sandboxPaths struct {
	state, client string
}

func paths(name string) (sandboxPaths, error) {
	base, err := config.Dir()
	if err != nil {
		return sandboxPaths{}, err
	}
	return sandboxPaths{filepath.Join(base, name), filepath.Join(base, config.File)}, nil
}
