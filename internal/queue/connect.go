package queue

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func options(s config.Settings) ([]nats.Option, error) {
	switch {
	case s.Queue == "":
		return nil, ErrNoQueue
	case !strings.HasPrefix(s.Queue, "tls://"):
		return nil, ErrInsecure
	}
	opts := []nats.Option{nats.RetryOnFailedConnect(true), nats.ReconnectWait(250 * time.Millisecond),
		nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}), nats.UserInfo(s.QueueUser, s.QueuePass)}
	if s.QueueCreds != "" {
		opts = append(opts, nats.UserCredentials(s.QueueCreds))
	}
	if s.QueueInbox != "" {
		opts = append(opts, nats.CustomInboxPrefix(s.QueueInbox))
	}
	if s.QueueCA != "" {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(s.QueueCA)) {
			return nil, errors.New("сертификат очереди в queue_ca не разобран")
		}
		opts = append(opts, nats.Secure(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}))
	}
	return opts, nil
}

// Check проверяет адрес очереди в настройках s без подключения. Возвращает
// ErrNoQueue без адреса и ErrInsecure без защищённого соединения.
func Check(s config.Settings) error {
	_, err := options(s)
	return err
}
