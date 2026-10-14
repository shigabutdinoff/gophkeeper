package record

import (
	"uuid"

	"github.com/nats-io/nats.go"
)

// Subject задаёт общее начало тем очереди для изменений записей.
const Subject = "changes.records"

// Change описывает изменение записи.
type Change struct {
	// Op задаёт операцию put, edit или delete.
	Op string `json:"op" validate:"oneof=put edit delete"`
	// Record задаёт идентификатор записи.
	Record string `json:"record"`
	// Note задаёт открытую пометку записи.
	Note string `json:"note" validate:"excludesrune=\x00"`
	// Secret содержит зашифрованные поля записи.
	Secret []byte `json:"secret" validate:"required_unless=Op delete,omitempty,min=1"`
	// Digest задаёт сумму зашифрованного файла записи.
	Digest string `json:"digest,omitempty" validate:"excluded_unless=Op put,omitempty,startswith=SHA-256=,len=52"`
	// Owner задаёт владельца из темы изменения после Parse.
	Owner string `json:"-"`
}

// Topic возвращает тему очереди для изменений владельца owner.
func Topic(owner string) string {
	return Subject + "." + owner
}

// FilesTopic возвращает тему запроса, которым владелец owner просит очередь
// открыть его корзину файлов.
func FilesTopic(owner string) string {
	return "files.open." + owner
}

// Put собирает изменение, которое создаёт запись владельца owner с пометкой
// note, и возвращает его вместе с идентификатором записи. Поля fields
// шифруются ключом к данным data.
func Put(data, owner string, fields map[string]string, note string) (*nats.Msg, string, error) {
	id := uuid.NewV7().String()
	msg, err := sealed("put", data, owner, id, fields, note, "")
	return msg, id, err
}
