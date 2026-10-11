package account

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

var pageSize = 100

// List возвращает записи владельца токена доступа access с секретами в
// порядке создания, читая их страницами. Занятый сервер опрашивается
// повторно.
func List(ctx context.Context, client *http.Client, s config.Settings, access string) ([]Entry, error) {
	var all []Entry
	for {
		var page []Entry
		path := fmt.Sprintf("rest/v1/records?select=id,note,secret&order=id&limit=%d", pageSize)
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
