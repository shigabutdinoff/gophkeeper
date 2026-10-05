package record

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKeys(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	_, sign, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err, "ключ устройства не создан")
	key := make([]byte, 32)
	_, err = rand.Read(key)
	require.NoError(t, err, "ключ к данным не создан")
	return base64.RawURLEncoding.EncodeToString(key), sign
}

func put(t *testing.T, data string, sign ed25519.PrivateKey) (*nats.Msg, string, change) {
	t.Helper()
	msg, id, err := Put(data, sign, "anna", "s3cret", "сайт example.com")
	require.NoError(t, err, "изменение не собрано")
	var c change
	require.NoError(t, json.Unmarshal(msg.Data, &c), "тело изменения не JSON")
	return msg, id, c
}

func open(data, record string, secret []byte) (map[string]string, error) {
	key, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nil, secret, []byte(record))
	if err != nil {
		return nil, err
	}
	var got map[string]string
	return got, json.Unmarshal(plain, &got)
}

func TestPutHidesSecrets(t *testing.T) {
	data, sign := newKeys(t)
	msg, id, c := put(t, data, sign)
	assert.Equal(t, Subject, msg.Subject, "изменение не в теме записей")
	assert.Equal(t, change{Op: "put", Record: id, Note: "сайт example.com", Secret: c.Secret}, c, "тело изменения не то")
	assert.Contains(t, string(msg.Data), "сайт example.com", "пометка не открыта")
	assert.NotContains(t, string(msg.Data), "anna", "логин ушёл открыто")
	assert.NotContains(t, string(msg.Data), "s3cret", "пароль ушёл открыто")
	got, err := open(data, id, c.Secret)
	require.NoError(t, err, "секрет не расшифрован")
	assert.Equal(t, map[string]string{"login": "anna", "password": "s3cret"}, got, "расшифрован не тот секрет")
	_, err = open(data, "другая", c.Secret)
	assert.Error(t, err, "секрет перенесён в другую запись")
	other, _ := newKeys(t)
	_, err = open(other, id, c.Secret)
	assert.Error(t, err, "секрет открыт чужим ключом")
}

func TestPutSigned(t *testing.T) {
	data, sign := newKeys(t)
	msg, _, _ := put(t, data, sign)
	key, err := base64.StdEncoding.DecodeString(msg.Header.Get("Gophkeeper-Key"))
	require.NoError(t, err, "открытый ключ не base64")
	signature, err := base64.StdEncoding.DecodeString(msg.Header.Get("Gophkeeper-Signature"))
	require.NoError(t, err, "подпись не base64")
	assert.Equal(t, sign.Public(), ed25519.PublicKey(key), "в заголовке чужой ключ")
	assert.True(t, ed25519.Verify(key, msg.Data, signature), "подпись не проходит")
	assert.False(t, ed25519.Verify(key, append(msg.Data, ' '), signature), "подпись прошла с другим телом")
}

func TestPutNewIDs(t *testing.T) {
	data, sign := newKeys(t)
	first, firstID, _ := put(t, data, sign)
	second, secondID, _ := put(t, data, sign)
	assert.NotEqual(t, firstID, secondID, "идентификатор записи повторился")
	assert.NotEmpty(t, first.Header.Get(nats.MsgIdHdr), "у изменения нет идентификатора")
	assert.NotEqual(t, first.Header.Get(nats.MsgIdHdr), second.Header.Get(nats.MsgIdHdr), "идентификатор изменения повторился")
}

func TestPutBadDataKey(t *testing.T) {
	_, sign := newKeys(t)
	for _, data := range []string{"не base64", base64.RawURLEncoding.EncodeToString(make([]byte, 31))} {
		_, _, err := Put(data, sign, "anna", "s3cret", "")
		assert.Error(t, err, "изменение собрано с неверным ключом %q", data)
	}
}
