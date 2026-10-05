package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Signup создаёт на сервере s учётную запись email с ключом входа из
// AuthKey вместо пароля. Email хранится без пробелов по краям и строчными
// буквами. Ключ приложения уходит в заголовке apikey.
func Signup(ctx context.Context, client *http.Client, s config.Settings, email, password string) error {
	endpoint, err := url.JoinPath(s.Server, "auth/v1/signup")
	if err != nil {
		return fmt.Errorf("адрес сервера %q: %w", s.Server, err)
	}
	key, err := AuthKey(email, password)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"email": normalize(email), "password": key})
	if err != nil {
		return fmt.Errorf("тело запроса: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("адрес сервера %q: %w", s.Server, err)
	}
	req.Header.Set("apikey", s.AppKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoAnswer, err)
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
	}()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	return failure(resp.Body)
}
