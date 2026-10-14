package server

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var pauses = []time.Duration{time.Second, 3 * time.Second}

func again(ctx context.Context, f func() error) error {
	return retry.New(retry.Context(ctx), retry.Attempts(uint(len(pauses)+1)), retry.LastErrorOnly(true), retry.RetryIf(lost),
		retry.DelayType(func(n uint, _ error, _ retry.DelayContext) time.Duration { return pauses[n-1] })).Do(f)
}

func lost(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if pg, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgerrcode.IsConnectionException(pg.Code) || pg.Code == pgerrcode.AdminShutdown || pg.Code == pgerrcode.CannotConnectNow
	}
	_, connect := errors.AsType[*pgconn.ConnectError](err)
	_, broken := errors.AsType[net.Error](err)
	return connect || broken || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
