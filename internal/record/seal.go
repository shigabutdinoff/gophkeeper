package record

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/chacha20poly1305"
)

func seal(data, record string, plain []byte) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("ключ к данным: %w", err)
	}
	x, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("ключ к данным: %w", err)
	}
	nonce := make([]byte, x.NonceSize(), x.NonceSize()+len(plain)+x.Overhead())
	rand.Read(nonce)
	return x.Seal(nonce, nonce, plain, []byte(record)), nil
}

func text(note string, fields map[string]string) bool {
	ok := utf8.ValidString(note)
	for _, v := range fields {
		ok = ok && utf8.ValidString(v)
	}
	return ok
}
