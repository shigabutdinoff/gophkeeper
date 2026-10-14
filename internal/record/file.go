package record

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/nats-io/nats.go"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// MaxFile задаёт наибольший размер файла записи в байтах.
	MaxFile = 50_000_000
	// MaxSealed задаёт наибольший размер зашифрованного файла в байтах.
	MaxSealed = MaxFile + chacha20poly1305.NonceSizeX + chacha20poly1305.Overhead
	fileBound = "/file"
)

// SealFile шифрует ключом к данным data содержимое plain файла записи id.
func SealFile(data, id string, plain []byte) ([]byte, error) {
	return seal(data, id+fileBound, plain)
}

// OpenFile расшифровывает ключом к данным data файл sealed записи id.
func OpenFile(data, id string, sealed []byte) ([]byte, error) {
	return unseal(data, id+fileBound, sealed)
}

// Bucket возвращает имя корзины файлов владельца owner, выведенное из ключа
// key. Без ключа имя корзины по владельцу не узнать.
func Bucket(key, owner string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(owner))
	return hex.EncodeToString(mac.Sum(nil))
}

// Digest возвращает сумму зашифрованного файла file в записи очереди.
func Digest(file []byte) string {
	sum := sha256.Sum256(file)
	return "SHA-256=" + base64.URLEncoding.EncodeToString(sum[:])
}

// PutFile собирает изменение, которое создаёт запись id владельца owner о
// зашифрованном файле file с его суммой. Поля fields шифруются, как в Put.
func PutFile(data, owner, id string, fields map[string]string, note string, file []byte) (*nats.Msg, error) {
	return sealed("put", data, owner, id, fields, note, Digest(file))
}
