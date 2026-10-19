package cli

import (
	"slices"
	"strings"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
)

func noted(list []account.Entry, note string) []account.Entry {
	want := strings.ToLower(note)
	return slices.DeleteFunc(list, func(e account.Entry) bool { return !strings.Contains(strings.ToLower(e.Note), want) })
}
