package record

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"uuid"

	"github.com/nats-io/nats.go"
)

// Subject задаёт тему очереди для изменений записей.
const Subject = "changes.records"

type change struct {
	Op     string `json:"op"`
	Record string `json:"record"`
	Note   string `json:"note"`
	Secret []byte `json:"secret"`
}

// Put собирает изменение, которое создаёт запись с пометкой note, и
// возвращает его вместе с идентификатором записи. Логин и пароль шифруются
// ключом к данным data, изменение подписывается ключом устройства sign.
func Put(data string, sign ed25519.PrivateKey, login, password, note string) (*nats.Msg, string, error) {
	id := uuid.NewV7().String()
	plain, err := json.Marshal(map[string]string{"login": login, "password": password})
	if err != nil {
		return nil, "", err
	}
	secret, err := seal(data, id, plain)
	if err != nil {
		return nil, "", err
	}
	msg := nats.NewMsg(Subject)
	if msg.Data, err = json.Marshal(change{"put", id, note, secret}); err != nil {
		return nil, "", err
	}
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	msg.Header.Set("Gophkeeper-Key", base64.StdEncoding.EncodeToString(sign.Public().(ed25519.PublicKey)))
	msg.Header.Set("Gophkeeper-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(sign, msg.Data)))
	return msg, id, nil
}
