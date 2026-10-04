package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/nats-io/nats-server/v2/server"
)

const handshakeSeconds float64 = 3

func serverOptions(state string, port int) (*server.Options, error) {
	tlsConfig, err := server.GenTLSConfig(&server.TLSConfigOpts{
		CertFile: filepath.Join(state, certFile), KeyFile: filepath.Join(state, keyFile),
	})
	if err != nil {
		return nil, fmt.Errorf("TLS очереди песочницы: %w", err)
	}
	admin, aerr := readPass(state, adminUser)
	client, cerr := readPass(state, clientUser)
	if err = errors.Join(aerr, cerr); err != nil {
		return nil, err
	}
	cfg, err := loadStream()
	if err != nil {
		return nil, fmt.Errorf("поток изменений: %w", err)
	}
	return &server.Options{
		ServerName: "gophkeeper-sandbox", Host: sandboxHost, Port: port,
		TLSConfig: tlsConfig, TLSTimeout: handshakeSeconds, AuthTimeout: handshakeSeconds,
		JetStream: true, StoreDir: filepath.Join(state, "jetstream"), PidFile: filepath.Join(state, pidFile),
		Users: []*server.User{{Username: adminUser, Password: string(admin)}, {
			Username: clientUser, Password: string(client), Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{clientSubject(cfg)}},
				Subscribe: &server.SubjectPermission{Allow: []string{queueInbox + ".>"}},
			},
		}},
	}, nil
}
