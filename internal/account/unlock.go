package account

import (
	"context"
	"errors"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Unlock возвращает ключ к данным для команд с данными. Без входа
// возвращает ErrLoggedOut. Без системного хранилища спрашивает у ask пароль
// для email входа, и тогда ключ живёт только в памяти.
func Unlock(ctx context.Context, client *http.Client, s config.Settings,
	ask func(email string) (string, error)) (string, error) {
	email, err := loadEmail(s.Server)
	if err != nil {
		return "", err
	}
	r, ok := recall()
	if !ok || r.Email != email || r.Server != s.Server {
		password, err := ask(email)
		if err != nil {
			return "", err
		}
		body, data, err := credentials(email, password)
		if err == nil {
			_, err = token(ctx, client, s, "password", body)
		}
		if err != nil {
			return "", err
		}
		return data, nil
	}
	t, err := refresh(ctx, client, s, r.Refresh)
	if errors.Is(err, ErrLoggedOut) {
		err = errors.Join(err, forget())
	}
	if err != nil {
		return "", err
	}
	r.Refresh = t.Refresh
	if err = keep(r); err != nil {
		return "", err
	}
	return r.Data, nil
}
