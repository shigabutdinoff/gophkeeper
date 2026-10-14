package account

import (
	"context"
	"net/http"
	"strings"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Logout завершает вход на сервере, где он был выполнен. Сертификату сервера
// доверяет по cfg, если cfg описывает этот сервер, иначе как при входе. Вход
// удаляется локально, даже если сервер не ответил или не доверенный.
func Logout(ctx context.Context, client *http.Client, cfg config.Settings) error {
	if r, ok := recall(); ok {
		if strings.TrimSuffix(cfg.Server, "/") == r.Server {
			r.Cert = cfg.ServerCert
		}
		_ = Trust(client, r.Cert)
		s := config.Settings{Server: r.Server, AppKey: r.AppKey}
		t, err := refresh(ctx, client, s, r.Refresh)
		if err == nil {
			_ = post(ctx, client, s, "auth/v1/logout?scope=local", t.Access, struct{}{}, nil)
		}
	}
	return forget()
}
