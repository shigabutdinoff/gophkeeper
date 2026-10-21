package main

import (
	"errors"
	"fmt"

	"github.com/nats-io/nkeys"
)

func newIssuer() (seed, public string, err error) {
	pair, err := nkeys.CreateAccount()
	if err != nil {
		return "", "", fmt.Errorf("ключ выдачи доступа к очереди: %w", err)
	}
	secret, serr := pair.Seed()
	public, perr := pair.PublicKey()
	return string(secret), public, errors.Join(serr, perr)
}
