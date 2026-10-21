package account

import (
	"context"
	"net/http"
	"uuid"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Entry описывает запись владельца на сервере.
type Entry struct {
	// ID задаёт идентификатор записи.
	ID string `json:"id"`
	// Note задаёт открытую пометку записи.
	Note string `json:"note"`
	// Secret содержит зашифрованные поля записи.
	Secret []byte `json:"secret"`
}

// Find возвращает запись id владельца токена доступа access с секретом.
// Чужую, несуществующую или неверно записанную запись отмечает ErrNotFound.
func Find(ctx context.Context, client *http.Client, s config.Settings, access, id string) (Entry, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	var list []Entry
	path := "rest/v1/records?select=id,note,secret&id=eq." + u.String()
	if err = call(ctx, client, s, true, http.MethodGet, path, access, nil, &list); err != nil {
		return Entry{}, err
	}
	if len(list) == 0 {
		return Entry{}, ErrNotFound
	}
	return list[0], nil
}
