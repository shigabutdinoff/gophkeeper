package account

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/zalando/go-keyring"
)

func forget() error {
	s, loadErr := load()
	err := keyring.Delete(service, user)
	if (s.Kept || loadErr != nil) && err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("ключ к данным остался в хранилище: %w", err)
	}
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("файл входа: %w", err)
	}
	return nil
}
