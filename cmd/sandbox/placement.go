package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type placement struct {
	client, aside string
	same          bool
}

func placeSettings(want []byte, source, clientPath string) (placement, error) {
	p := placement{client: clientPath}
	got, err := os.ReadFile(clientPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return p, fmt.Errorf("чтение файла настроек: %w", err)
	}
	if p.same = bytes.Equal(got, want); p.same {
		return p, nil
	}
	if got != nil && !bytes.HasPrefix(got, []byte(markerPrefix)) {
		p.aside = asidePath(clientPath, source)
	}
	return p, saveSettings(cmp.Or(p.aside, clientPath), want)
}

func asidePath(clientPath, source string) string {
	return filepath.Join(filepath.Dir(clientPath), "config."+source+".yaml")
}

var _ fmt.Stringer = placement{}

func (p placement) String() string {
	switch {
	case p.same:
		return fmt.Sprintf("файл настроек %q уже содержит эти настройки", p.client)
	case p.aside != "":
		return fmt.Sprintf("в %q другие настройки, клиент продолжит работать с ними. Новые настройки лежат в %q",
			p.client, p.aside)
	}
	return fmt.Sprintf("файл настроек клиента записан: %q", p.client)
}
