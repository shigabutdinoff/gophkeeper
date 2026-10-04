package main

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotRunning(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "слушатель не открыт")
	addr := l.Addr().String()
	require.NoError(t, l.Close(), "слушатель не закрыт")
	_, refused := nats.Connect("nats://"+addr, nats.NoReconnect())
	timeout := &net.OpError{Op: "dial", Err: os.ErrDeadlineExceeded}
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"отказ":       {refused, true},
		"нет сервера": {nats.ErrNoServers, true},
		"тайм-аут":    {timeout, false},
		"чужая":       {errors.New("права"), false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, notRunning(c.err), "ошибка %v", c.err)
		})
	}
}
