package account

import (
	"context"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func credentials(email, password string) (map[string]string, string, error) {
	auth, data, err := Keys(email, password)
	return map[string]string{"email": normalize(email), "password": auth}, data, err
}

func enter(ctx context.Context, client *http.Client, s config.Settings,
	body map[string]string, data string) (bool, error) {
	t, err := token(ctx, client, s, false, "password", body)
	if err != nil {
		return false, err
	}
	return save(record{Server: s.Server, Email: body["email"], Data: data, Refresh: t.Refresh, AppKey: s.AppKey, Access: t.Access, User: t.User.ID})
}

// Login входит на сервер s по email и паролю и запоминает вход. Возвращает
// false, если системного хранилища секретов нет и пароль будет нужен при
// каждой команде с данными.
func Login(ctx context.Context, client *http.Client, s config.Settings, email, password string) (bool, error) {
	body, data, err := credentials(email, password)
	if err != nil {
		return false, err
	}
	return enter(ctx, client, s, body, data)
}
