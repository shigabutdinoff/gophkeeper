package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
	"github.com/shigabutdinoff/gophkeeper/internal/storage"
)

func fileStore(o online) *storage.Client {
	t, _ := httpClient.Transport.(*http.Transport)
	t = cmp.Or(t, http.DefaultTransport.(*http.Transport)).Clone()
	t.ResponseHeaderTimeout = httpClient.Timeout
	return storage.New(&http.Client{Transport: t}, o.s.Server, o.s.Region, o.s.AppKey, o.keys.Access)
}

func fetch(ctx context.Context, files *storage.Client, owner, id string) ([]byte, error) {
	data, err := files.Get(ctx, owner, id, record.MaxSealed)
	switch {
	case errors.Is(err, storage.ErrNoObject):
		return nil, errDamaged
	case err != nil:
		return nil, account.Reason(fmt.Errorf("%w: %w", account.ErrNoAnswer, err))
	}
	return data, nil
}
