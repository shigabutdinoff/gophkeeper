package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func serve(ctx context.Context, srv *http.Server, l net.Listener, grace time.Duration) error {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		wait, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		if srv.Shutdown(wait) != nil {
			_ = srv.Close()
		}
	})
	defer stop()
	if err := srv.ServeTLS(l, "", ""); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-done
	return nil
}
