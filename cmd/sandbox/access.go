package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nats-io/nats.go"
)

const (
	stopSubject = "sandbox.stop"
	adminUser   = "admin"
	clientUser  = "client"
	caFile      = "ca.pem"
	certFile    = "cert.pem"
	keyFile     = "key.pem"
	pidFile     = "sandbox.pid"
)

func readPass(state, user string) ([]byte, error) {
	pass, err := os.ReadFile(filepath.Join(state, user))
	if err != nil {
		return nil, fmt.Errorf("пароль %s: %w", user, err)
	}
	return pass, nil
}

func sandboxUser(state, user string) (nats.Option, error) {
	pass, err := readPass(state, user)
	if err != nil {
		return nil, err
	}
	ca := nats.RootCAs(filepath.Join(state, caFile))
	return func(o *nats.Options) error {
		return errors.Join(nats.UserInfo(user, string(pass))(o), ca(o), nats.Timeout(queueTimeout)(o))
	}, nil
}

func sandboxAccess(state string) (queueAccess, error) {
	admin, err := sandboxUser(state, adminUser)
	client, cerr := sandboxUser(state, clientUser)
	return queueAccess{sandboxQueue, admin, client}, errors.Join(err, cerr)
}
