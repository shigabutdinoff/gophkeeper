package account

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"

	"github.com/go-resty/resty/v2"
)

func busy(r *resty.Response, _ error) bool {
	return r != nil && slices.Contains([]int{http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout}, r.StatusCode())
}

type quiet struct{}

func (quiet) Errorf(string, ...any) {}
func (quiet) Warnf(string, ...any)  {}
func (quiet) Debugf(string, ...any) {}

// Reason готовит ошибку сервера для показа. Отказ соединения становится
// ErrUnavailable, таймаут ErrNoAnswer, остальные ошибки не меняются.
func Reason(err error) error {
	var ne net.Error
	switch {
	case !errors.Is(err, ErrNoAnswer):
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ErrNoAnswer
	}
	return ErrUnavailable
}
