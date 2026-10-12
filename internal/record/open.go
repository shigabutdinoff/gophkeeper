package record

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

var errSecret = errors.New("секрет записи не расшифрован")

// Open расшифровывает ключом к данным data поля из секрета записи с
// идентификатором id.
func Open(data, id string, secret []byte) (map[string]string, error) {
	plain, err := unseal(data, id, secret)
	var got map[string]string
	if err != nil || json.Unmarshal(plain, &got) != nil {
		return nil, errSecret
	}
	return got, nil
}

func unseal(data, bound string, secret []byte) ([]byte, error) {
	key, kerr := base64.RawURLEncoding.DecodeString(data)
	x, xerr := chacha20poly1305.NewX(key)
	if kerr != nil || xerr != nil || len(secret) < x.NonceSize() {
		return nil, errSecret
	}
	return x.Open(nil, secret[:x.NonceSize()], secret[x.NonceSize():], []byte(bound))
}
