package record

import (
	"encoding/json"
	"uuid"

	"github.com/nats-io/nats.go"
)

// Edit собирает изменение, которое меняет поля fields и пометку записи id
// у владельца owner. Ключ к данным data используется так же, как в Put.
func Edit(data, owner, id string, fields map[string]string, note string) (*nats.Msg, error) {
	return sealed("edit", data, owner, id, fields, note)
}

// Delete собирает изменение, которое удаляет запись id у владельца owner.
func Delete(owner, id string) (*nats.Msg, error) {
	return build(owner, Change{Op: "delete", Record: id})
}

func sealed(op, data, owner, id string, fields map[string]string, note string) (*nats.Msg, error) {
	if !text(note, fields) {
		return nil, ErrNotText
	}
	plain, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	secret, err := seal(data, id, plain)
	if err != nil {
		return nil, err
	}
	return build(owner, Change{Op: op, Record: id, Note: note, Secret: secret})
}

func build(owner string, c Change) (*nats.Msg, error) {
	msg := nats.NewMsg(Topic(owner))
	var err error
	if msg.Data, err = json.Marshal(c); err != nil {
		return nil, err
	}
	if len(msg.Data) > maxChange {
		return nil, ErrTooLarge
	}
	msg.Header.Set(nats.MsgIdHdr, uuid.NewV7().String())
	return msg, nil
}
