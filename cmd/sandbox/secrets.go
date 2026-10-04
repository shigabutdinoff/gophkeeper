package main

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func sandboxSecrets(state string) error {
	_, err := os.Stat(filepath.Join(state, certFile))
	if errors.Is(err, fs.ErrNotExist) {
		err = issueCerts(state)
	}
	if err != nil {
		return err
	}
	if err = once(filepath.Join(state, adminUser)); err != nil {
		return err
	}
	return once(filepath.Join(state, clientUser))
}

func once(path string) error {
	data, err := os.ReadFile(path)
	if len(data) > 0 || (err != nil && !errors.Is(err, fs.ErrNotExist)) {
		return err
	}
	return writeFile(path, []byte(rand.Text()))
}
