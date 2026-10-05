package account

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Unlock возвращает ключи для команд с данными, при сбое сервера берёт их
// из хранилища секретов. Без входа возвращает ErrLoggedOut. Без хранилища
// спрашивает у ask пароль для email входа и регистрирует новый ключ.
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
	if len(r.Sign) != ed25519.SeedSize {
		return Secrets{}, ErrLoggedOut
	}
	t, err := refresh(ctx, client, s, r.Refresh)
	switch {
	case errors.Is(err, ErrLoggedOut):
		return Secrets{}, errors.Join(err, forget())
	case err == nil:
		r.Refresh = t.Refresh
		if err = keep(r); err != nil {
			return Secrets{}, err
		}
	}
	return Secrets{r.Data, ed25519.NewKeyFromSeed(r.Sign)}, nil
}
