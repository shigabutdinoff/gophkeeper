package server

import (
	"net/url"
	"strings"
	"uuid"
)

func owned(q url.Values) (map[string]any, bool) {
	where := map[string]any{}
	for _, name := range []string{"user_id", "id"} {
		v, eq := strings.CutPrefix(q.Get(name), "eq.")
		u, err := uuid.Parse(v)
		if !eq || err != nil {
			return nil, false
		}
		where[name] = u.String()
	}
	return where, true
}

func valid(in stored) bool {
	_, err := uuid.Parse(in.ID)
	_, uerr := uuid.Parse(in.UserID)
	return err == nil && uerr == nil
}
