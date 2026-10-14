package account

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

var retryWait, retryMax = 200 * time.Millisecond, 2 * time.Second

func post(ctx context.Context, client *http.Client, s config.Settings,
	path, bearer string, body, reply any) error {
	return call(ctx, client, s, false, http.MethodPost, path, bearer, body, reply)
}

func call(ctx context.Context, client *http.Client, s config.Settings, retry bool,
	method, path, bearer string, body, reply any) error {
	if _, err := url.Parse(s.Server); err != nil {
		return fmt.Errorf("адрес сервера %q: %w", s.Server, err)
	}
	var fail struct {
		Code string `json:"error_code"`
	}
	c := resty.NewWithClient(client).SetResponseBodyLimit(1 << 24).SetBaseURL(s.Server).SetLogger(quiet{})
	if retry {
		c.SetRetryCount(3).SetRetryWaitTime(retryWait).SetRetryMaxWaitTime(retryMax).AddRetryCondition(busy)
	}
	resp, err := c.R().SetContext(ctx).SetHeader("apikey", s.AppKey).SetBody(body).SetError(&fail).
		ForceContentType("application/json").SetAuthToken(bearer).SetResult(reply).Execute(method, path)
	switch {
	case errors.As(err, new(*tls.CertificateVerificationError)):
		return ErrUntrusted
	case err != nil && (resp == nil || resp.RawResponse == nil):
		return fmt.Errorf("%w: %w", ErrNoAnswer, err)
	case resp.IsError():
		return failure(fail.Code)
	case err != nil:
		return ErrServer
	}
	return nil
}
