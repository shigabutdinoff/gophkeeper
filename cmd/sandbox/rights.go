package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const queueInbox = "_INBOX.pub"

func checkClient(url string, cfg jetstream.StreamConfig, opts ...nats.Option) error {
	quiet := nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {})
	nc, err := nats.Connect(url, append(opts, nats.CustomInboxPrefix(queueInbox), quiet)...)
	if err != nil {
		return fmt.Errorf("подключение клиента к очереди: %w", err)
	}
	defer nc.Close()
	probes := map[string]func(string) error{
		clientSubject(cfg): func(s string) error { _, serr := nc.SubscribeSync(s); return serr },
		jetstream.DefaultAPIPrefix + "STREAM.INFO." + cfg.Name: func(s string) error { return nc.Publish(s, nil) },
	}
	for subject, run := range probes {
		if err = errors.Join(run(subject), nc.Flush()); err != nil {
			return fmt.Errorf("проверка прав клиента очереди: %w", err)
		}
		last := nc.LastError()
		if !errors.Is(last, nats.ErrPermissionViolation) || !strings.Contains(last.Error(), strconv.Quote(subject)) {
			return errors.New("клиент очереди может читать или менять чужие изменения, ограничьте его права по README")
		}
	}
	return nil
}
