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
	saltPrefix   = "gophkeeper "
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	keyLen       = 32
)

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// AuthKey выводит из email и пароля ключ входа, который сервер получает
// вместо пароля. По ключу нельзя восстановить ни пароль, ни ключ к данным.
func AuthKey(email, password string) (string, error) {
	master := argon2.IDKey([]byte(password), []byte(saltPrefix+normalize(email)), argonTime, argonMemory, argonThreads, keyLen)
	key, err := hkdf.Key(sha256.New, master, nil, authLabel, keyLen)
	if err != nil {
		return "", fmt.Errorf("вывод ключа входа: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
}
