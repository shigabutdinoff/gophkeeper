package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func sandboxSecrets(state string) ([]byte, error) {
	if _, err := once(filepath.Join(state, "cert.pem"), func() ([]byte, error) { return issueCerts(state) }); err != nil {
		return nil, err
	}
	admin, err := once(filepath.Join(state, "admin"), password)
	if err != nil {
		return nil, err
	}
	client, err := once(filepath.Join(state, "client"), password)
	if err != nil {
		return nil, err
	}
	conf := fmt.Appendf(nil, "ADMIN_PASSWORD: %q\nCLIENT_PASSWORD: %q\n", admin, client)
	return client, writeFile(filepath.Join(state, "passwords.conf"), conf)
}

func once(path string, create func() ([]byte, error)) ([]byte, error) {
	data, err := os.ReadFile(path)
	if len(data) > 0 || (err != nil && !errors.Is(err, fs.ErrNotExist)) {
		return data, err
	}
	if data, err = create(); err == nil {
		err = writeFile(path, data)
	}
	return data, err
}

func password() ([]byte, error) {
	return []byte(rand.Text()), nil
}
