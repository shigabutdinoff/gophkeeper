package account

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

type session struct {
	Server, Email string
	Kept          bool
}

func sessionPath() (string, error) {
	dir, err := config.Dir()
	return filepath.Join(dir, "session.yaml"), err
}

func save(r record) (bool, error) {
	path, err := sessionPath()
	if err != nil {
		return false, err
	}
	kept := keep(r) == nil
	data, err := yaml.Marshal(session{Server: r.Server, Email: r.Email, Kept: kept})
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o700)
	}
	if err == nil {
		err = os.WriteFile(path, data, 0o600)
	}
	if err != nil {
		return false, fmt.Errorf("запись входа %q: %w", path, err)
	}
	return kept, nil
}
