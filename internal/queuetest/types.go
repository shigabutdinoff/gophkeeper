package queuetest

import (
	"crypto/ecdsa"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

// Queue описывает очередь изменений для тестов.
type Queue struct {
	// Client задаёт настройки клиента, вход ему даёт As.
	Client config.Settings
	// Admin задаёт настройки администратора без ограничений.
	Admin config.Settings
	// Server обслуживает очередь после Run или Start.
	Server *server.Server
	// Supabase задаёт адрес сервера, чьи токены принимает очередь.
	Supabase string
	// Seed задаёт ключ выдачи доступа к очереди.
	Seed string
	key  *ecdsa.PrivateKey
	opts *server.Options
}
