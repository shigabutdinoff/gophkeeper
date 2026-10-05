package account

import (
	"context"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Logout завершает вход на сервере, где он был выполнен, без файла настроек.
// Сессии других устройств работают дальше. Запись хранилища и файл входа
// удаляются, даже если сервер не ответил. Повторный выход проходит без ошибки.
func Logout(ctx context.Context, client *http.Client) error {
	if r, ok := recall(); ok {
		s := config.Settings{Server: r.Server, AppKey: r.AppKey}
		t, err := refresh(ctx, client, s, r.Refresh)
		if err == nil {
			post(ctx, client, s, "auth/v1/logout?scope=local", t.Access, struct{}{}, nil)
		}
	}
	return forget()
}
