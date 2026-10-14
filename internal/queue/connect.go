package queue

import (
	"cmp"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func options(s config.Settings) ([]nats.Option, error) {
	if err := address(s.Queue, ErrNoQueue); err != nil {
		return nil, err
	}
	opts := []nats.Option{nats.RetryOnFailedConnect(true), nats.ReconnectWait(250 * time.Millisecond), nats.UserInfo(s.QueueUser, s.QueuePass),
		nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {})}
	if s.QueueCreds != "" {
		opts = append(opts, nats.UserCredentials(s.QueueCreds))
	}
	if s.QueueCA != "" {
		t, err := secure(s.QueueCA)
		if err != nil {
			return nil, err
		}
		opts = append(opts, nats.Secure(t))
	}
	return opts, nil
}

// Check проверяет адреса очереди в настройках s без подключения. Возвращает
// ErrNoQueue или ErrNoLeaf без адреса и ErrInsecure без защищённого
// соединения.
func Check(s config.Settings) error {
	_, err := options(s)
	return cmp.Or(err, address(s.QueueLeaf, ErrNoLeaf))
}
