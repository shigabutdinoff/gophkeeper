package main

import (
	"cmp"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"time"
)

const certLifetime = 87600 * time.Hour

func signCert(tmpl, parent *x509.Certificate,
	signer *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl.NotBefore = time.Now()
	tmpl.NotAfter = tmpl.NotBefore.Add(certLifetime)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, cmp.Or(parent, tmpl), key.Public(), cmp.Or(signer, key))
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	return cert, key, err
}
