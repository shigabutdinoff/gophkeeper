package main

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"path/filepath"
)

func issueCerts(state string) error {
	ca, caKey, err := signCert(&x509.Certificate{
		Subject: pkix.Name{CommonName: "gophkeeper-sandbox-ca"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}, nil, nil)
	if err != nil {
		return err
	}
	leaf, key, err := signCert(&x509.Certificate{
		Subject: pkix.Name{CommonName: "gophkeeper-sandbox"}, DNSNames: []string{"localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage:    x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, ca, caKey)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	for _, f := range []struct {
		name, kind string
		der        []byte
	}{
		{keyFile, "PRIVATE KEY", der}, {caFile, "CERTIFICATE", ca.Raw}, {certFile, "CERTIFICATE", leaf.Raw},
	} {
		if err = writeFile(filepath.Join(state, f.name), pem.EncodeToMemory(&pem.Block{Type: f.kind, Bytes: f.der})); err != nil {
			return err
		}
	}
	return nil
}
