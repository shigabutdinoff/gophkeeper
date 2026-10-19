package server

import (
	"net/url"
	"strconv"
	"strings"
	"uuid"

	"gorm.io/gorm/clause"
)

const maxRows = 1000

func filter(q url.Values) (where clause.Expression, limit int, ok bool) {
	limit, all := maxRows, []clause.Expression{clause.Expr{SQL: "true"}}
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 0 {
			return nil, 0, false
		}
		limit = min(n, maxRows)
	}
	for _, f := range []struct {
		name  string
		parse func(string) (clause.Expression, bool)
	}{{"id", byID}, {"note", byNote}} {
		if !q.Has(f.name) {
			continue
		}
		e, ok := f.parse(q.Get(f.name))
		if !ok {
			return nil, 0, false
		}
		all = append(all, e)
	}
	return clause.And(all...), limit, true
}

func byID(v string) (clause.Expression, bool) {
	op, id, _ := strings.Cut(v, ".")
	u, err := uuid.Parse(id)
	cmp := map[string]string{"eq": "id = ?", "gt": "id > ?"}[op]
	if err != nil || cmp == "" {
		return nil, false
	}
	return clause.Expr{SQL: cmp, Vars: []any{u.String()}}, true
}
