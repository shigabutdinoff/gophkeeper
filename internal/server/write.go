package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func (a *auth) write(s Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.writer(w, r, s) {
			return
		}
		var in stored
		where, ok := owned(r.URL.Query())
		if r.Method != http.MethodDelete {
			ok = json.NewDecoder(r.Body).Decode(&in) == nil && (ok || r.Method == http.MethodPost && valid(in))
		}
		if !ok {
			respond(w, http.StatusBadRequest, syntax)
			return
		}
		rows, err := a.save(r, in, where)
		if pg, ok := errors.AsType[*pgconn.PgError](err); ok && pg.Code == pgerrcode.ForeignKeyViolation {
			w.Header().Set("Proxy-Status", "PostgREST; error=23503")
			respond(w, http.StatusConflict, orphan)
			return
		}
		if err != nil {
			respond(w, http.StatusInternalServerError, internal)
			return
		}
		status, out := http.StatusOK, []map[string]string{}
		for _, row := range rows {
			if row.Inserted {
				status = http.StatusCreated
			}
			out = append(out, map[string]string{"user_id": row.UserID})
		}
		body, _ := json.Marshal(out)
		respond(w, status, string(body))
	}
}
