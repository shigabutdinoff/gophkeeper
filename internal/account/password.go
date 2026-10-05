package account

import (
	"context"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func byPassword(ctx context.Context, client *http.Client, s config.Settings,
	email string, ask func(email string) (string, error)) (Secrets, error) {
	password, err := ask(email)
	if err != nil {
		return Secrets{}, err
	}
	body, data, err := credentials(email, password)
	if err != nil {
		return Secrets{}, err
	}
	_, sign, err := authorize(ctx, client, s, body)
	return Secrets{data, sign}, err
}
