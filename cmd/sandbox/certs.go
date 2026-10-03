package main

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"net"
	"path/filepath"
)

func issueCerts(state string) ([]byte, error) {
	ca, caKey, err := signCert(&x509.Certificate{
		Subject: pkix.Name{CommonName: "gophkeeper-sandbox-ca"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}, nil, nil)
	if err != nil {
		return nil, err
	}
	leaf, key, err := signCert(&x509.Certificate{
		Subject: pkix.Name{CommonName: "nats"}, DNSNames: []string{"localhost", "nats"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage:    x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, ca, caKey)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pemBlock("CERTIFICATE", leaf.Raw), errors.Join(writeFile(filepath.Join(state, "key.pem"), pemBlock("PRIVATE KEY", der)),
		writeFile(filepath.Join(state, "ca.pem"), pemBlock("CERTIFICATE", ca.Raw)))
}

func pemBlock(kind string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
}
