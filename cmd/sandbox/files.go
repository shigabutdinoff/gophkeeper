package main

import (
	"errors"
	"fmt"
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

func chownLike(refs []string, paths ...string) error {
	var uid, gid int
	for _, ref := range refs {
		info, err := os.Stat(ref)
		if err != nil {
			return fmt.Errorf("владелец каталога настроек: %w", err)
		}
		var ok bool
		if uid, gid, ok = owner(info); !ok {
			return nil
		}
		if uid != 0 {
			break
		}
	}
	errs := make([]error, len(paths))
	for i, path := range paths {
		errs[i] = os.Chown(path, uid, gid)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("владелец файла настроек: %w", err)
	}
	return nil
}
