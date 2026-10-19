package cli

import (
	"context"
	"maps"
	"slices"
	"strings"
	"uuid"

	"github.com/shigabutdinoff/gophkeeper/internal/account"
	"github.com/shigabutdinoff/gophkeeper/internal/record"
)

const unsentMark, copyMark = "не отправлено", "сервер недоступен, показаны записи с устройства"

func merge(list []account.Entry, left map[string]record.Change) []account.Entry {
	all := map[string]account.Entry{}
	for _, e := range list {
		all[e.ID] = e
	}
	for id, c := range left {
		delete(all, id)
		if c.Op != "delete" {
			all[id] = account.Entry{ID: c.Record, Note: c.Note, Secret: c.Secret}
		}
	}
	return slices.SortedFunc(maps.Values(all), func(a, b account.Entry) int { return strings.Compare(a.ID, b.ID) })
}

func pick(ctx context.Context, o online, id string) (account.Entry, string, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return account.Entry{}, "", account.ErrNotFound
	}
	c, unsent := o.left[u.String()]
	switch {
	case unsent && c.Op == "delete":
		return account.Entry{}, "", account.ErrNotFound
	case unsent:
		return account.Entry{ID: c.Record, Note: c.Note, Secret: c.Secret}, "\n" + unsentMark, nil
	case o.keys.Down != nil:
		e, err := copyEntry(o, u.String())
		return e, "\n" + copyMark, err
	}
	e, err := account.Find(ctx, httpClient, o.s, o.keys.Access, u.String())
	return e, "", err
}
