package account

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Secrets содержит ключи для команд с данными.
type Secrets struct {
	// Data задаёт ключ к данным.
	Data string
	// Sign задаёт закрытый ключ устройства для подписи изменений.
	Sign ed25519.PrivateKey
}

func register(ctx context.Context, client *http.Client, s config.Settings, access string) (ed25519.PrivateKey, error) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("ключ устройства: %w", err)
	}
	body := map[string]string{"public_key": base64.StdEncoding.EncodeToString(public)}
	if err = post(ctx, client, s, "rest/v1/device_keys", access, body, nil); err != nil {
		return nil, fmt.Errorf("регистрация ключа устройства: %w", err)
	}
	return private, nil
}

func authorize(ctx context.Context, client *http.Client, s config.Settings, body map[string]string) (tokens, ed25519.PrivateKey, error) {
	t, err := token(ctx, client, s, "password", body)
	if err != nil {
		return t, nil, err
	}
	sign, err := register(ctx, client, s, t.Access)
	return t, sign, err
}
