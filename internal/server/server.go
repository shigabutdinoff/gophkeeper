package server

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrNoCert сообщает, что сертификат или ключ сервера не заданы.
var ErrNoCert = errors.New("нужны сертификат и ключ сервера")

// Serve принимает защищённые соединения на l, пока не отменён ctx.
// После отмены отвечает на начатые запросы не дольше десяти секунд.
// Без ключа подписи токенов, базы данных или корзины файлов приём не
// начинает.
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
	defer func() { _ = db.Close() }()
	if a.files, err = bucket(context.WithoutCancel(ctx), s); err != nil {
		return err
	}
	srv := &http.Server{Handler: a.routes(s), ReadTimeout: cmp.Or(s.readTimeout, 30*time.Second),
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}},
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}}
	return serve(ctx, srv, l, cmp.Or(s.grace, 10*time.Second))
}
