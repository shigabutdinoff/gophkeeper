package cli

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestRestoreOnInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("сигнал себе в Windows не отправить")
	}
	restored := make(chan int, 1)
	codes := make(chan int, 1)
	setVar(t, &getState, func(int) (*term.State, error) { return &term.State{}, nil })
	setVar(t, &restore, func(fd int, _ *term.State) error { restored <- fd; return nil })
	setVar(t, &exit, func(code int) { codes <- code })
	stop := restoreOnInterrupt(7)
	defer stop()
	self, err := os.FindProcess(os.Getpid())
	require.NoError(t, err, "текущий процесс не найден")
	require.NoError(t, self.Signal(os.Interrupt), "сигнал не отправлен")
	select {
	case code := <-codes:
		assert.Equal(t, 7, <-restored, "режим терминала не восстановлен")
		assert.Equal(t, interruptCode, code, "неверный код выхода")
	case <-time.After(time.Second):
		t.Fatal("прерывание не перехвачено")
	}
}

func TestRestoreOnInterruptNoTerminal(t *testing.T) {
	restoreOnInterrupt(int(os.Stdin.Fd()))()
}
