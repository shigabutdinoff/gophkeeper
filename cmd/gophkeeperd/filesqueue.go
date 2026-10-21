package main

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

func filesQueue(s serverSettings) (jetstream.JetStream, int64, error) {
	limit, err := humanize.ParseBytes(s.FilesLimit)
	if err != nil || limit == 0 || limit > 1<<62 {
		return nil, 0, fmt.Errorf("предел корзины файлов %q не разобран, нужен объём вроде 1GB", s.FilesLimit)
	}
	nc, err := queue.Connect(s.queue(), nats.MaxReconnects(-1))
	if err != nil {
		return nil, 0, err
	}
	js, err := jetstream.New(nc)
	return js, int64(limit), err
}
