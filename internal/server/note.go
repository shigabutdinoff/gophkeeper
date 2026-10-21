package server

import (
	"strings"

	"gorm.io/gorm/clause"
)

func byNote(v string) (clause.Expression, bool) {
	like, ok := strings.CutPrefix(v, "ilike.")
	if !ok {
		return nil, false
	}
	return clause.Expr{SQL: "note ILIKE ?", Vars: []any{strings.ReplaceAll(like, "*", "%")}}, true
}
