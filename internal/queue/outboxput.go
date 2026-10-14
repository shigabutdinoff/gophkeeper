package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Put сохраняет изменение msg в очереди устройства с тем же идентификатором
// сообщения. Отмена ctx не прерывает сохранение.
func (o *Outbox) Put(ctx context.Context, msg *nats.Msg) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deadline)
	defer cancel()
	if _, err := o.js.PublishMsg(ctx, copyChange(local+msg.Subject, msg)); err != nil {
		return fmt.Errorf("очередь устройства: %w", err)
	}
	return nil
}

func (o *Outbox) send(ctx context.Context, seq uint64) error {
	raw, err := o.box.GetMsg(ctx, seq)
	if errors.Is(err, jetstream.ErrMsgNotFound) {
		return nil
	}
	if err == nil {
		err = publish(ctx, o.nc, copyChange(strings.TrimPrefix(raw.Subject, local), &nats.Msg{Header: raw.Header, Data: raw.Data}))
	}
	if err == nil {
		err = o.box.DeleteMsg(context.WithoutCancel(ctx), seq)
	}
	return err
}

func copyChange(subject string, msg *nats.Msg) *nats.Msg {
	m := nats.NewMsg(subject)
	m.Header.Set(nats.MsgIdHdr, msg.Header.Get(nats.MsgIdHdr))
	m.Data = msg.Data
	return m
}
