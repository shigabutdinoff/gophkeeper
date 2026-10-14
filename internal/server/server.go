package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Settings задаёт сертификат сервера, ключи проекта и хранение входа.
type Settings struct {
	// Cert и Key задают пути к файлам сертификата и ключа сервера.
	Cert, Key string
	// AppKey и ServerKey задают ключ приложения и служебный ключ.
	AppKey, ServerKey string
	// TokenKey и Database задают файл ключа подписи токенов и адрес базы.
	TokenKey, Database string
}

// ErrNoCert сообщает, что сертификат или ключ сервера не заданы.
var ErrNoCert = errors.New("нужны сертификат и ключ сервера")

var readTimeout, grace = 30 * time.Second, 10 * time.Second
var operations = http.NotFoundHandler()

// Serve принимает защищённые соединения на l, пока не отменён ctx.
// После отмены отвечает на начатые запросы не дольше десяти секунд.
// Без ключа подписи токенов или базы данных приём не начинает.
func Serve(ctx context.Context, l net.Listener, s Settings) error {
	if s.Cert == "" || s.Key == "" {
		return ErrNoCert
	}
	cert, err := tls.LoadX509KeyPair(s.Cert, s.Key)
	if err != nil {
		return fmt.Errorf("сертификат и ключ сервера не прочитаны: %w", err)
	}
	a, err := open(ctx, s)
	if err != nil {
		return err
	}
	db, _ := a.db.DB()
	defer db.Close()
	srv := &http.Server{Handler: a.routes(s), ReadTimeout: readTimeout,
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}},
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}}
	return serve(ctx, srv, l)
}
