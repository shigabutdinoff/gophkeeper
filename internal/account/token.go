package account

import (
	"context"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

type tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func token(ctx context.Context, client *http.Client, s config.Settings,
	grant string, body map[string]string) (tokens, error) {
	var t tokens
	err := post(ctx, client, s, "auth/v1/token?grant_type="+grant, "", body, &t)
	return t, err
}

func refresh(ctx context.Context, client *http.Client, s config.Settings, rt string) (tokens, error) {
	return token(ctx, client, s, "refresh_token", map[string]string{"refresh_token": rt})
}
