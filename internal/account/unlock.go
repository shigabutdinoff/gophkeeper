package account

import (
	"context"
	"errors"
	"net/http"

	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Unlock возвращает ключ и токен доступа. При сбое сервера берёт их из
// хранилища секретов с причиной в Down, без хранилища спрашивает пароль у
// ask. Без входа ErrLoggedOut, при недоверенном сертификате ErrUntrusted.
func Unlock(ctx context.Context, client *http.Client, s config.Settings,
	ask func(email string) (string, error)) (Secrets, error) {
	email, err := loadEmail(s.Server)
	if err != nil {
		return Secrets{}, err
	}
	r, ok := recall()
	if !ok || r.Email != email || r.Server != s.Server {
		return byPassword(ctx, client, s, email, ask)
	}
	t, err := refresh(ctx, client, s, r.Refresh)
	switch {
	case errors.Is(err, ErrLoggedOut):
		return Secrets{}, errors.Join(err, forget())
	case errors.Is(err, ErrUntrusted):
		return Secrets{}, err
	case err == nil:
		r.Refresh, r.Access, r.User = t.Refresh, t.Access, t.User.ID
		if err = keep(r, keyring.Set); err != nil {
			return Secrets{}, err
		}
	}
	return Secrets{Data: r.Data, Access: r.Access, User: r.User, Down: err}, nil
}
