package worker

import (
	"context"
	"time"
)

// Grace задаёт, сколько Run ждёт ответа сервера после отмены контекста.
var Grace = 10 * time.Second

func graceful(ctx context.Context) (context.Context, context.CancelFunc) {
	grace := Grace
	work, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, func() { time.AfterFunc(grace, cancel) })
	return work, func() { stop(); cancel() }
}
