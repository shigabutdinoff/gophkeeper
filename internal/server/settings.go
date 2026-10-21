package server

import (
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Settings задаёт сертификат сервера, ключи проекта и хранение входа.
type Settings struct {
	// Cert и Key задают пути к файлам сертификата и ключа сервера.
	Cert, Key string
	// AppKey и ServerKey задают ключ приложения и служебный ключ.
	AppKey, ServerKey string
	// TokenKey и Database задают файл ключа подписи токенов и адрес базы.
	TokenKey, Database string
	// Log получает строки о внутренних ошибках чтения записей.
	Log io.Writer
	// Queue подключает сервер к очереди, где лежит корзина файлов.
	Queue jetstream.JetStream
	// FilesLimit задаёт предел объёма корзины файлов в байтах.
	FilesLimit int64

	readTimeout, grace time.Duration
	operations         http.Handler
	pauses             []time.Duration
}
