package account

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

const pageSize = 100

// List возвращает записи владельца токена access по порядку создания, читая
// страницами и повторяя опрос занятого сервера. Непустой note оставляет
// записи с этой подстрокой в пометке без учёта регистра.
func List(ctx context.Context, client *http.Client, s config.Settings, access, note string) ([]Entry, error) {
	var all []Entry
	for {
		var page []Entry
		path := fmt.Sprintf("rest/v1/records?select=id,note,secret&order=id&limit=%d", pageSize)
		if note != "" {
			path += "&note=" + url.QueryEscape("ilike.*"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(note)+"*")
		}
		if len(all) > 0 {
			path += "&id=gt." + all[len(all)-1].ID
		}
		if err := call(ctx, client, s, true, http.MethodGet, path, access, nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}
