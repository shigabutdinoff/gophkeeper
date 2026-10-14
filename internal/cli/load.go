package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

func load(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("файл %s не найден", path)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, record.MaxFile+1))
	switch {
	case err != nil:
		return nil, fmt.Errorf("файл %s не найден", path)
	case len(data) > record.MaxFile:
		return nil, errors.New("файлы больше 50 МБ не поддерживаются")
	}
	return data, nil
}
