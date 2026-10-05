package record

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

func seal(data, record string, plain []byte) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("ключ к данным: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ключ к данным: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("шифр записи: %w", err)
	}
	return gcm.Seal(nil, nil, plain, []byte(record)), nil
}
