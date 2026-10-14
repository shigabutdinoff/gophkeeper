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
	limit, where = maxRows, clause.Expr{SQL: "true"}
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 0 {
			return nil, 0, false
		}
		limit = min(n, maxRows)
	}
	if !q.Has("id") {
		return where, limit, true
	}
	op, id, _ := strings.Cut(q.Get("id"), ".")
	u, err := uuid.Parse(id)
	cmp := map[string]string{"eq": "id = ?", "gt": "id > ?"}[op]
	if err != nil || cmp == "" {
		return nil, 0, false
	}
	return clause.Expr{SQL: cmp, Vars: []any{u.String()}}, limit, true
}
