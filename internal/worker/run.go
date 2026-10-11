package worker

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/avast/retry-go/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Server описывает сервер, к которому обработчик применяет изменения.
type Server struct {
	// URL задаёт адрес сервера.
	URL string
	// Key задаёт ключ доступа к серверу.
	Key string
	// Client отправляет запросы к серверу.
	Client *http.Client
	// Log принимает строки журнала без секретов.
	Log *log.Logger
}

// Run применяет изменения из очереди js к серверу s, пока не отменён ctx.
// После отмены текущее изменение доводится до сервера не дольше Grace.
// Возвращает ошибку, только если соединение с очередью закрыто.
func Run(ctx context.Context, js jetstream.JetStream, s Server) error {
	work, cancel := graceful(ctx)
	defer cancel()
	s.Log.Print("обработчик очереди изменений запущен")
	err := retry.New(retry.Context(ctx), retry.UntilSucceeded(), retry.MaxDelay(maxDelay),
		retry.RetryIf(func(err error) bool { return !errors.Is(err, nats.ErrConnectionClosed) }),
		retry.OnRetry(func(_ uint, err error) { s.Log.Printf("очередь изменений: %v", err) }),
	).Do(func() error { return consume(ctx, work, js, s) })
	if ctx.Err() != nil {
		return nil
	}
	return err
}
