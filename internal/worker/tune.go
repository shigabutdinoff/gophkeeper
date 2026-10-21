package worker

import (
	"cmp"
	"time"

	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

// Grace задаёт, сколько Run ждёт ответа сервера после отмены контекста.
const Grace = 10 * time.Second

type tuning struct {
	attempt, ackWait, heartbeat, maxDelay, grace time.Duration
	maxSealed                                    uint64
}

func (t tuning) tuned() tuning {
	t.attempt = cmp.Or(t.attempt, 5*time.Second)
	t.ackWait = cmp.Or(t.ackWait, 30*time.Second)
	t.heartbeat = cmp.Or(t.heartbeat, 5*time.Second)
	t.maxDelay = cmp.Or(t.maxDelay, 5*time.Second)
	t.grace = cmp.Or(t.grace, Grace)
	t.maxSealed = cmp.Or(t.maxSealed, record.MaxSealed)
	return t
}
