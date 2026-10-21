package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

const noOwner = "23503"

type row struct {
	UserID string `json:"user_id"`
}

func query(ctx context.Context, s Server, method, table string, filter map[string]string, body any) ([]row, error) {
	var rows []row
	resp, err := resty.NewWithClient(s.Client).SetResponseBodyLimit(1<<16).SetBaseURL(s.URL).R().SetContext(ctx).
		SetHeader("apikey", s.Key).SetAuthToken(s.Key).SetHeader("Prefer", "return=representation,resolution=merge-duplicates").
		SetQueryParams(filter).SetQueryParam("select", "user_id").SetBody(body).SetResult(&rows).ForceContentType("application/json").
		Execute(method, "rest/v1/"+table)
	if resp != nil && resp.RawResponse != nil && resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		code := strings.TrimPrefix(resp.Header().Get("Proxy-Status"), "PostgREST; error=")
		if code == noOwner {
			return nil, fmt.Errorf("%w, владельца нет на сервере", errSkipped)
		}
		return nil, fmt.Errorf("ответ сервера %d, код %.64q", resp.StatusCode(), code)
	}
	return rows, err
}
