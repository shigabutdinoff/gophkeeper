package record

import "errors"

var (
	// ErrForged сообщает, что изменение пришло без владельца или не разобрано.
	ErrForged = errors.New("изменение отброшено")
	// ErrTooLarge сообщает, что изменение больше 64 КиБ.
	ErrTooLarge = errors.New("запись больше 64 КиБ, сократите запись или пометку")
	// ErrNotText сообщает, что поле или пометка записи не текст Юникода.
	ErrNotText = errors.New("поля и пометка записи должны быть текстом в кодировке UTF-8")
)

const maxChange = 1 << 16
