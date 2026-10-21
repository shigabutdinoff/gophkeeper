package cli

import (
	"errors"
	"uuid"

	"github.com/spf13/cobra"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/queue"
)

func lookup(c *cobra.Command, id string, unchecked bool) (online, account.Entry, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return online{}, account.Entry{}, account.ErrNotFound
	}
	e := account.Entry{ID: u.String()}
	o, err := connect(c)
	if err == nil {
		err = queue.Check(o.s)
	}
	if err != nil {
		return o, e, err
	}
	if err = o.keys.Down; err == nil {
		e, err = account.Find(c.Context(), httpClient, o.s, o.keys.Access, e.ID)
	}
	if unchecked && errors.Is(err, account.ErrNoAnswer) {
		return o, account.Entry{ID: u.String()}, nil
	}
	return o, e, err
}
