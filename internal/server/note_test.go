package server

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func notes(t *testing.T, r *running, query, token string) []string {
	t.Helper()
	status, body := r.records(t, query, token)
	require.Equal(t, http.StatusOK, status, "поиск отклонён: %s", body)
	var got []string
	for _, e := range list(t, body) {
		got = append(got, e.Note)
	}
	return got
}

func TestRecordsNote(t *testing.T) {
	r := start(t)
	anna, bob := r.user(t), r.user(t)
	ids := []string{r.put(t, anna.User.ID, "Сбербанк, зарплатная"), r.put(t, anna.User.ID, "сайт example.com"), r.put(t, anna.User.ID, "100%_почта")}
	assert.Equal(t, []string{"Сбербанк, зарплатная"}, notes(t, r, "?note=ilike.*сбербанк*", anna.Access), "поиск без учёта регистра нашёл не то")
	assert.Empty(t, notes(t, r, "?note=ilike.*сбербанк*", bob.Access), "чужая запись нашлась")
	assert.Empty(t, notes(t, r, "?note=ilike.*сайт", anna.Access), "шаблон без звёздочки в конце совпал с началом")
	assert.Equal(t, []string{"100%_почта"}, notes(t, r, `?note=ilike.*0\%25\_*`, anna.Access), "экранированные знаки не совпали буквально")
	assert.Len(t, notes(t, r, "?note=ilike.*а*&id=gt."+slices.Min(ids), anna.Access), 2, "поиск не учёл страницу по id")
	assert.Len(t, notes(t, r, "?note=ilike.*а*&limit=1", anna.Access), 1, "поиск не учёл предел")
}

func TestNoteIndex(t *testing.T) {
	dsn := database(t)
	p := migrated(t, dsn)
	_, err := p.Up(context.Background())
	require.NoError(t, err, "схема не создана")
	conn, n := connect(t, dsn), 0
	q := "SELECT count(*) FROM pg_indexes WHERE indexname = 'records_note_trgm' AND schemaname = current_schema()"
	require.NoError(t, conn.QueryRow(context.Background(), q).Scan(&n), "индексы не прочитаны")
	assert.Equal(t, 1, n, "индекса поиска по пометке нет")
	_, err = p.DownTo(context.Background(), 3)
	require.NoError(t, err, "миграция индекса не откатилась")
	require.NoError(t, conn.QueryRow(context.Background(), q).Scan(&n), "индексы не прочитаны")
	assert.Zero(t, n, "индекс остался после отката")
}
