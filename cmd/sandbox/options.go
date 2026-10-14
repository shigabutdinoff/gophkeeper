package main

import (
	"fmt"
	"path/filepath"

	"github.com/nats-io/nats-server/v2/server"
)

const handshakeSeconds float64 = 3

func serverOptions(state string, port int) (*server.Options, string, error) {
	tlsConfig, err := server.GenTLSConfig(&server.TLSConfigOpts{
		CertFile: filepath.Join(state, certFile), KeyFile: filepath.Join(state, keyFile),
	})
	if err != nil {
		return nil, "", fmt.Errorf("TLS очереди песочницы: %w", err)
	}
	admin, err := readPass(state, adminUser)
	if err != nil {
		return nil, "", err
	}
	seed, public, err := newIssuer()
	if err != nil {
		return nil, "", err
	}
	return &server.Options{
		ServerName: "gophkeeper-sandbox", Host: sandboxHost, Port: port,
		TLSConfig: tlsConfig, TLSTimeout: handshakeSeconds, AuthTimeout: handshakeSeconds,
		NoSigs: true, JetStream: true, StoreDir: filepath.Join(state, "jetstream"), PidFile: filepath.Join(state, pidFile),
		Users:       []*server.User{{Username: adminUser, Password: string(admin)}},
		AuthCallout: &server.AuthCallout{Issuer: public, Account: server.DEFAULT_GLOBAL_ACCOUNT, AuthUsers: []string{adminUser}},
	}, seed, nil
}

func launchQueue(state string) (*server.Server, string, error) {
	opts, seed, err := serverOptions(state, sandboxPort)
	if err != nil {
		return nil, "", err
	}
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, "", fmt.Errorf("создание очереди песочницы: %w", err)
	}
	s.ConfigureLogger()
	s.Start()
	return s, seed, nil
}
