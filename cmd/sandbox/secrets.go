package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
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
	return once(filepath.Join(state, adminUser), []byte(rand.Text()))
}

func once(path string, secret []byte) error {
	data, err := os.ReadFile(path)
	if len(data) > 0 || (err != nil && !errors.Is(err, fs.ErrNotExist)) {
		return err
	}
	return writeFile(path, secret)
}

func ownKeys(state string, own bool) error {
	if !own {
		return nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return errors.Join(once(filepath.Join(state, serviceKeyFile), []byte(rand.Text())), once(filepath.Join(state, appKeyFile), []byte(rand.Text())),
		once(filepath.Join(state, tokenKeyFile), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
}
