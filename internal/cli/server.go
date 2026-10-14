package cli

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const serverEnv = "GOPHKEEPER_SERVER"

var httpClient = &http.Client{Timeout: 9 * time.Second}

func resolve(flag string) (config.Settings, error) {
	dir, err := config.Dir()
	if err != nil {
		return config.Settings{}, err
	}
	path := filepath.Join(dir, config.File)
	s, err := config.Load[config.Settings](path)
	if err = cmp.Or(err, account.Trust(httpClient, s.ServerCert)); err != nil {
		return config.Settings{}, err
	}
	server := cmp.Or(os.Getenv(serverEnv), flag, s.Server)
	if server == "" {
		return config.Settings{}, fmt.Errorf("не задан адрес сервера: укажите переменную %s, флаг --server или server в файле %s", serverEnv, path)
	}
	if !strings.HasPrefix(server, "https://") {
		return config.Settings{}, config.ErrInsecureServer
	}
	server = strings.TrimSuffix(server, "/")
	if s.AppKey == "" || server != strings.TrimSuffix(s.Server, "/") {
		return config.Settings{}, fmt.Errorf("не задан ключ приложения для сервера %s: укажите server и app_key в файле %s", server, path)
	}
	s.Server = server
	return s, nil
}
