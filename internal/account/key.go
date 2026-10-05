package account

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	authLabel    = "gophkeeper auth"
	dataLabel    = "gophkeeper data"
	saltPrefix   = "gophkeeper "
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	keyLen       = 32
)

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Keys выводит из email и пароля ключ входа, который сервер получает
// вместо пароля, и ключ к данным. По ключу входа нельзя восстановить ни
// пароль, ни ключ к данным.
func Keys(email, password string) (auth, data string, err error) {
	master := argon2.IDKey([]byte(password), []byte(saltPrefix+normalize(email)), argonTime, argonMemory, argonThreads, keyLen)
	if auth, err = derive(master, authLabel); err != nil {
		return "", "", err
	}
	data, err = derive(master, dataLabel)
	return auth, data, err
}

func derive(master []byte, label string) (string, error) {
	key, err := hkdf.Key(sha256.New, master, nil, label, keyLen)
	if err != nil {
		return "", fmt.Errorf("вывод ключа: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
}
