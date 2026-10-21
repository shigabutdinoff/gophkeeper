package record

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"
)

const anna = "0199b5e4-0000-7000-8000-00000000a11a"

func newKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "ключ к данным не создан")
	return base64.RawURLEncoding.EncodeToString(key)
}

func put(t *testing.T, data string) (*nats.Msg, string, Change) {
	t.Helper()
	msg, id, err := Put(data, anna, map[string]string{"login": "anna", "password": "s3cret"}, "сайт example.com")
	require.NoError(t, err, "изменение не собрано")
	var c Change
	require.NoError(t, json.Unmarshal(msg.Data, &c), "тело изменения не JSON")
	return msg, id, c
}

func open(data, record string, secret []byte) (map[string]string, error) {
	key, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	x, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	plain, err := x.Open(nil, secret[:chacha20poly1305.NonceSizeX], secret[chacha20poly1305.NonceSizeX:], []byte(record))
	if err != nil {
		return nil, err
	}
	var got map[string]string
	return got, json.Unmarshal(plain, &got)
}

func TestPutHidesSecrets(t *testing.T) {
	data := newKey(t)
	msg, id, c := put(t, data)
	assert.Equal(t, Change{Op: "put", Record: id, Note: "сайт example.com", Secret: c.Secret}, c, "тело изменения не то")
	assert.Contains(t, string(msg.Data), "сайт example.com", "пометка не открыта")
	assert.NotContains(t, string(msg.Data), "anna", "логин ушёл открыто")
	assert.NotContains(t, string(msg.Data), "s3cret", "пароль ушёл открыто")
	got, err := open(data, id, c.Secret)
	require.NoError(t, err, "секрет не расшифрован")
	assert.Equal(t, map[string]string{"login": "anna", "password": "s3cret"}, got, "расшифрован не тот секрет")
	_, err = open(data, "другая", c.Secret)
	assert.Error(t, err, "секрет перенесён в другую запись")
	other := newKey(t)
	_, err = open(other, id, c.Secret)
	assert.Error(t, err, "секрет открыт чужим ключом")
}

func TestPutOwnerTopic(t *testing.T) {
	msg, _, _ := put(t, newKey(t))
	assert.Equal(t, "changes.records."+anna, msg.Subject, "изменение не в теме владельца")
	assert.Equal(t, msg.Subject, Topic(anna), "тема владельца собрана иначе")
	assert.Empty(t, msg.Header.Get("Gophkeeper-Key"), "в изменении остался ключ устройства")
	assert.Empty(t, msg.Header.Get("Gophkeeper-Signature"), "в изменении осталась подпись")
}

func TestPutNewIDs(t *testing.T) {
	data := newKey(t)
	first, firstID, _ := put(t, data)
	second, secondID, _ := put(t, data)
	assert.NotEqual(t, firstID, secondID, "идентификатор записи повторился")
	assert.NotEmpty(t, first.Header.Get(nats.MsgIdHdr), "у изменения нет идентификатора")
	assert.NotEqual(t, first.Header.Get(nats.MsgIdHdr), second.Header.Get(nats.MsgIdHdr), "идентификатор изменения повторился")
}

func TestPutBadDataKey(t *testing.T) {
	for _, data := range []string{"не base64", base64.RawURLEncoding.EncodeToString(make([]byte, 31))} {
		_, _, err := Put(data, anna, map[string]string{"login": "anna"}, "")
		assert.Error(t, err, "изменение собрано с неверным ключом %q", data)
	}
}

func TestOpen(t *testing.T) {
	data := newKey(t)
	_, id, c := put(t, data)
	got, err := Open(data, id, c.Secret)
	require.NoError(t, err, "секрет не открыт")
	assert.Equal(t, map[string]string{"login": "anna", "password": "s3cret"}, got, "не те логин и пароль")
}

func TestOpenCard(t *testing.T) {
	data := newKey(t)
	card := map[string]string{"type": "card", "number": "4111 1111 1111 1111", "expiry": "12/28", "holder": "ANNA IVANOVA", "code": "123"}
	msg, id, err := Put(data, anna, card, "Сбербанк, зарплатная")
	require.NoError(t, err, "карта не собрана")
	c, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "карта не прошла проверку")
	got, err := Open(data, id, c.Secret)
	require.NoError(t, err, "карта не открыта")
	assert.Equal(t, card, got, "карта открыта не та")
}

func TestOpenText(t *testing.T) {
	data := newKey(t)
	text := map[string]string{"type": "text", "text": "abcd-1234\nefgh-5678\n\n"}
	msg, err := Edit(data, anna, id, text, "")
	require.NoError(t, err, "текст не собран")
	c, err := Parse(msg.Subject, msg.Data)
	require.NoError(t, err, "текст не прошёл проверку")
	got, err := Open(data, id, c.Secret)
	require.NoError(t, err, "текст не открыт")
	assert.Equal(t, text, got, "текст изменился")
}

func TestOpenOldLogin(t *testing.T) {
	data := newKey(t)
	secret, err := seal(data, id, []byte(`{"login":"anna","password":"s3cret"}`))
	require.NoError(t, err, "старый секрет не собран")
	got, err := Open(data, id, secret)
	require.NoError(t, err, "старый секрет не открыт")
	assert.Equal(t, map[string]string{"login": "anna", "password": "s3cret"}, got, "старый секрет открыт не так")
}

func TestOpenRejects(t *testing.T) {
	data, other := newKey(t), newKey(t)
	_, id, c := put(t, data)
	cases := map[string]struct {
		data, id string
		secret   []byte
	}{
		"чужой ключ":        {other, id, c.Secret},
		"чужая запись":      {data, "0199b2c4-6f1e-7a3b-9c2d-1e2f3a4b5c6d", c.Secret},
		"обрезанный секрет": {data, id, c.Secret[:5]},
		"неверный ключ":     {"!", id, c.Secret},
	}
	for name, tc := range cases {
		_, err := Open(tc.data, tc.id, tc.secret)
		assert.Error(t, err, "секрет открыт: %s", name)
	}
}

func TestSealRejectsBrokenText(t *testing.T) {
	data := newKey(t)
	_, _, err := Put(data, anna, map[string]string{"type": "text", "text": "код \xff"}, "")
	assert.ErrorIs(t, err, ErrNotText, "текст не в UTF-8 принят и был бы искажён")
	_, _, err = Put(data, anna, map[string]string{"login": "anna"}, "пометка \xd0")
	assert.ErrorIs(t, err, ErrNotText, "пометка не в UTF-8 принята и была бы искажена")
	_, err = Edit(data, anna, anna, map[string]string{"password": "\xd0"}, "")
	assert.ErrorIs(t, err, ErrNotText, "правка с полем не в UTF-8 принята")
	_, _, err = Put(data, anna, map[string]string{"text": "строка\nвторая 🤔"}, "пометка")
	assert.NoError(t, err, "текст в UTF-8 отклонён")
}

func TestSealFile(t *testing.T) {
	data := newKey(t)
	random := make([]byte, 1<<20)
	rand.Read(random)
	for name, plain := range map[string][]byte{"случайные байты": random, "пустой файл": {}} {
		sealed, err := SealFile(data, id, plain)
		require.NoError(t, err, "файл не зашифрован: %s", name)
		assert.NotContains(t, string(sealed), string(random[:16]), "файл ушёл открыто: %s", name)
		got, err := OpenFile(data, id, sealed)
		require.NoError(t, err, "файл не расшифрован: %s", name)
		assert.Equal(t, plain, append([]byte{}, got...), "файл изменился: %s", name)
	}
	_, err := SealFile("!", id, random)
	assert.Error(t, err, "файл зашифрован неверным ключом")
}

func TestOpenFileRejects(t *testing.T) {
	data, other := newKey(t), newKey(t)
	sealed, err := SealFile(data, id, []byte("ключ SSH"))
	require.NoError(t, err, "файл не зашифрован")
	_, _, c := put(t, data)
	secret, err := seal(data, id, []byte(`{"login":"anna"}`))
	require.NoError(t, err, "секрет не собран")
	_, err = Open(data, id, sealed)
	assert.Error(t, err, "файл открыт как секрет записи")
	cases := map[string]struct {
		data, id string
		sealed   []byte
	}{
		"секрет записи":   {data, id, secret},
		"чужой ключ":      {other, id, sealed},
		"чужая запись":    {data, c.Record, sealed},
		"обрезанный файл": {data, id, sealed[:len(sealed)-1]},
		"короче nonce":    {data, id, sealed[:5]},
		"неверный ключ":   {"!", id, sealed},
	}
	for name, tc := range cases {
		_, err := OpenFile(tc.data, tc.id, tc.sealed)
		assert.Error(t, err, "файл открыт: %s", name)
	}
}
