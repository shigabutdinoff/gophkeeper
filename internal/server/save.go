package server

import (
	"net/http"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	owner    = clause.Returning{Columns: []clause.Column{{Name: "user_id"}}}
	inserted = clause.Returning{Columns: []clause.Column{{Name: "user_id"}, {Name: "xmax = 0 AS inserted", Raw: true}}}
	upsert   = clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "id"}},
		DoUpdates: append(clause.AssignmentColumns([]string{"note", "secret"}), clause.Assignment{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("now()")})}
)

func (a *auth) save(r *http.Request, in stored, where map[string]any) ([]stored, error) {
	db := a.db.WithContext(r.Context())
	var rows []stored
	switch r.Method {
	case http.MethodPost:
		rows = append(rows, in)
		return rows, db.Clauses(inserted, upsert).Create(&rows).Error
	case http.MethodPatch:
		return rows, db.Model(&rows).Clauses(owner).Where(where).Updates(map[string]any{"note": in.Note, "secret": in.Secret, "updated_at": gorm.Expr("now()")}).Error
	}
	return rows, db.Clauses(owner).Where(where).Delete(&rows).Error
}
