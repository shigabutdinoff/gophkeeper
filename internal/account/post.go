package account

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func post(ctx context.Context, client *http.Client, s config.Settings,
	path, bearer string, body, reply any) error {
	if _, err := url.Parse(s.Server); err != nil {
		return fmt.Errorf("адрес сервера %q: %w", s.Server, err)
	}
	var fail struct {
		Code string `json:"error_code"`
	}
	req := resty.NewWithClient(client).SetResponseBodyLimit(1<<16).SetBaseURL(s.Server).R().SetContext(ctx).
		SetHeader("apikey", s.AppKey).SetBody(body).SetError(&fail).ForceContentType("application/json").SetAuthToken(bearer).SetResult(reply)
	resp, err := req.Post(path)
	switch {
	case err != nil && (resp == nil || resp.RawResponse == nil):
		return fmt.Errorf("%w: %w", ErrNoAnswer, err)
	case resp.IsError():
		return failure(fail.Code)
	case err != nil:
		return ErrServer
	}
	return nil
}
