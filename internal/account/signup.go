package account

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Signup создаёт на сервере s учётную запись email с ключом входа из Keys
// вместо пароля и запоминает вход, как Login. Email хранится без пробелов
// по краям и строчными буквами. Ключ приложения уходит в заголовке apikey.
func Signup(ctx context.Context, client *http.Client, s config.Settings, email, password string) (bool, error) {
	body, data, err := credentials(email, password)
	if err != nil {
		return false, err
	}
	if err = post(ctx, client, s, "auth/v1/signup", "", body, nil); err != nil {
		return false, err
	}
	saved, err := enter(ctx, client, s, body, data)
	if err != nil {
		return false, fmt.Errorf("учётная запись создана, войдите командой gophkeeper login: %w", err)
	}
	return saved, nil
}
