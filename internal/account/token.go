package account

import (
	"context"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

type tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	User    struct {
		ID string `json:"id"`
	} `json:"user"`
}

func token(ctx context.Context, client *http.Client, s config.Settings,
	retry bool, grant string, body map[string]string) (tokens, error) {
	var t tokens
	err := call(ctx, client, s, retry, http.MethodPost, "auth/v1/token?grant_type="+grant, "", body, &t)
	return t, err
}

func refresh(ctx context.Context, client *http.Client, s config.Settings, rt string) (tokens, error) {
	return token(ctx, client, s, true, "refresh_token", map[string]string{"refresh_token": rt})
}
