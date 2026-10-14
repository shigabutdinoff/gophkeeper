package queue

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"
)

func address(url string, missing error) error {
	switch {
	case url == "":
		return missing
	case !strings.HasPrefix(url, "tls://"):
		return ErrInsecure
	}
	return nil
}

func secure(ca string) (*tls.Config, error) {
	t := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca == "" {
		return t, nil
	}
	t.RootCAs = x509.NewCertPool()
	if !t.RootCAs.AppendCertsFromPEM([]byte(ca)) {
		return nil, errors.New("сертификат очереди в queue_ca не разобран")
	}
	return t, nil
}
