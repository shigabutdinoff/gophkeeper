package cli

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

const interruptCode = 130

var (
	getState = term.GetState
	restore  = term.Restore
	exit     = os.Exit
)

func restoreOnInterrupt(fd int) (stop func()) {
	state, err := getState(fd)
	if err != nil {
		return func() {}
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			restore(fd, state)
			exit(interruptCode)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sig)
		close(done)
	}
}
