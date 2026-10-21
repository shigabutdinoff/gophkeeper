package cli

import (
	"errors"
	"os"
	"path/filepath"
)

func pruneCopy(dir string, have map[string]bool) error {
	entries, err := os.ReadDir(dir)
	errs := []error{err}
	for _, e := range entries {
		if !have[e.Name()] {
			errs = append(errs, os.Remove(filepath.Join(dir, e.Name())))
		}
	}
	return errors.Join(errs...)
}
