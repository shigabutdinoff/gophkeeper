package main

import (
	"errors"
	"io/fs"
	"os"
)

func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	err := os.WriteFile(tmp, data, 0o600)
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		if rerr := os.Remove(tmp); !errors.Is(rerr, fs.ErrNotExist) {
			err = errors.Join(err, rerr)
		}
	}
	return err
}
