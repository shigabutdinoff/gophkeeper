package cli

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const serverEnv = "GOPHKEEPER_SERVER"

func resolve(flag string) (config.Settings, error) {
	dir, err := config.Dir()
	if err != nil {
		return config.Settings{}, err
	}
	path := filepath.Join(dir, config.File)
	s, err := config.Load(path)
	if err != nil {
		return config.Settings{}, err
	}
	server := cmp.Or(os.Getenv(serverEnv), flag, s.Server)
	if server == "" {
		return config.Settings{}, fmt.Errorf("не задан адрес сервера: укажите переменную %s, флаг --server или server в файле %s", serverEnv, path)
	}
	if !strings.HasPrefix(server, "https://") {
		return config.Settings{}, errors.New("сервер должен работать по защищённому соединению (https)")
	}
	if s.AppKey == "" || strings.TrimSuffix(server, "/") != strings.TrimSuffix(s.Server, "/") {
		return config.Settings{}, fmt.Errorf("не задан ключ приложения для сервера %s: укажите server и app_key в файле %s", server, path)
	}
	s.Server = server
	return s, nil
}
