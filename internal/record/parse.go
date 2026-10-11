package record

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"github.com/go-playground/validator/v10"
)

var valid = validator.New()

var reasons = map[string]string{"Op": "неизвестная операция", "Secret": "пустой секрет", "Note": "пометка содержит символ U+0000"}

// Parse разбирает изменение data из темы subject и ставит владельца темы в
// Owner. Идентификаторы возвращаются в каноническом виде. Тему без владельца
// или неразборчивое изменение отмечает ErrForged.
func Parse(subject string, data []byte) (Change, error) {
	tail, _ := strings.CutPrefix(subject, Subject+".")
	owner, err := uuid.Parse(tail)
	var c Change
	if err != nil {
		return Change{}, fmt.Errorf("%w: тема без владельца", ErrForged)
	}
	if json.Unmarshal(data, &c) != nil {
		return Change{}, fmt.Errorf("%w: тело не JSON", ErrForged)
	}
	id, err := uuid.Parse(c.Record)
	var bad validator.ValidationErrors
	switch {
	case len(data) > maxChange:
		return Change{}, fmt.Errorf("%w: больше 64 КиБ", ErrForged)
	case err != nil:
		return Change{}, fmt.Errorf("%w: запись не UUID", ErrForged)
	case errors.As(valid.Struct(c), &bad):
		return Change{}, fmt.Errorf("%w: %s", ErrForged, reasons[bad[0].Field()])
	}
	c.Owner, c.Record = owner.String(), id.String()
	return c, nil
}
