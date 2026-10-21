package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const putQuery = "?on_conflict=user_id,id&select=user_id"

func ownedQuery(owner, id string) string {
	return "?select=user_id&user_id=eq." + owner + "&id=eq." + id
}

func fields(owner, id, note string) map[string]string {
	return map[string]string{"id": id, "user_id": owner, "note": note, "secret": "c2VjcmV0", "updated_at": "now"}
}

func (r *running) write(t *testing.T, method, query, apikey, token string, body any) (int, string, http.Header) {
	t.Helper()
	return liveSupabase{server: "https://" + r.addr, client: r.client()}.send(t, method, "rest/v1/records"+query, writeHeader(apikey, token), body)
}

func (r *running) service(t *testing.T, method, query string, body any) string {
	t.Helper()
	status, got, h := r.write(t, method, query, "service-key", "service-key", body)
	assert.Equal(t, "application/json", h.Get("Content-Type"), "тип ответа записи не тот")
	return answer(status, masked(t, got))
}

type saved struct {
	note, secret string
	updated      time.Time
}

func (r *running) saved(t *testing.T, owner, id string) (saved, bool) {
	t.Helper()
	var s saved
	err := connect(t, r.dsn).QueryRow(context.Background(), "SELECT note, secret, updated_at FROM records WHERE user_id = $1 AND id = $2",
		owner, id).Scan(&s.note, &s.secret, &s.updated)
	return s, err == nil
}

func (r *running) count(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, connect(t, r.dsn).QueryRow(context.Background(), "SELECT count(*) FROM records").Scan(&n), "записи не посчитаны")
	return n
}

func TestWriteSaves(t *testing.T) {
	r := start(t)
	anna, boris := r.user(t), r.user(t)
	id := uuid.New().String()
	assert.Equal(t, reference(t, "write_created.txt"), r.service(t, http.MethodPost, putQuery, fields(anna.User.ID, id, "почта")),
		"сохранение новой записи не равно эталону Supabase")
	first, ok := r.saved(t, anna.User.ID, id)
	require.True(t, ok, "запись не сохранена")
	assert.Equal(t, saved{"почта", "c2VjcmV0", first.updated}, first, "сохранены не те поля")
	body := fields(anna.User.ID, id, "работа")
	body["secret"] = "bmV3"
	assert.Equal(t, reference(t, "write_merged.txt"), r.service(t, http.MethodPost, putQuery, body),
		"повторное сохранение не равно эталону Supabase")
	second, _ := r.saved(t, anna.User.ID, id)
	assert.Equal(t, []string{"работа", "bmV3"}, []string{second.note, second.secret}, "повторное сохранение не заменило поля")
	assert.True(t, second.updated.After(first.updated), "время изменения не выросло")
	assert.Equal(t, 1, r.count(t), "повторное сохранение добавило строку")
	r.service(t, http.MethodPost, putQuery, fields(boris.User.ID, id, "чужая"))
	assert.Equal(t, 2, r.count(t), "сохранение с чужим идентификатором не создало запись Бориса")
	kept, _ := r.saved(t, anna.User.ID, id)
	assert.Equal(t, "работа", kept.note, "сохранение Бориса изменило запись Анны")
	status, got := r.records(t, "?select=id,note,secret&id=eq."+id, anna.Access)
	assert.Equal(t, answer(http.StatusOK, `[{"id":"`+id+`","note":"работа","secret":"bmV3"}]`), answer(status, got), "чтение не отдало сохранённую запись")
	r.stop()
	require.NoError(t, r.wait(t), "сервер остановился с ошибкой")
	r.serve(t)
	status, got = r.records(t, "?select=id,note,secret&id=eq."+id, anna.Access)
	assert.Equal(t, answer(http.StatusOK, `[{"id":"`+id+`","note":"работа","secret":"bmV3"}]`), answer(status, got), "запись не пережила перезапуск")
}

func TestWriteOwned(t *testing.T) {
	r := start(t)
	anna, boris := r.user(t), r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		assert.Equal(t, reference(t, "write_none.txt"), r.service(t, method, ownedQuery(boris.User.ID, id), fields(boris.User.ID, id, "чужая")),
			"изменение Бориса нашло запись Анны: %s", method)
	}
	kept, ok := r.saved(t, anna.User.ID, id)
	assert.True(t, ok && kept.note == "почта", "изменение Бориса тронуло запись Анны")
	assert.Equal(t, reference(t, "write_patched.txt"), r.service(t, http.MethodPatch, ownedQuery(anna.User.ID, id), fields(boris.User.ID, uuid.New().String(), "работа")),
		"изменение не равно эталону Supabase")
	edited, ok := r.saved(t, anna.User.ID, id)
	assert.True(t, ok && edited.note == "работа", "изменение не применилось к записи владельца")
	assert.Equal(t, 1, r.count(t), "изменение сменило владельца или идентификатор записи")
	assert.Equal(t, reference(t, "write_deleted.txt"), r.service(t, http.MethodDelete, ownedQuery(anna.User.ID, id), nil),
		"удаление не равно эталону Supabase")
	assert.Equal(t, 0, r.count(t), "удаление не убрало запись")
	assert.Equal(t, reference(t, "write_none.txt"), r.service(t, http.MethodDelete, ownedQuery(anna.User.ID, id), nil), "повторное удаление не равно эталону Supabase")
}

func TestWriteRefused(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	other, err := loadKey(tokenKey(t))
	require.NoError(t, err, "чужой ключ не прочитан")
	forged, err := other.sign("https://"+r.addr+"/auth/v1", anna.User.ID, uuid.New().String(), time.Now().Add(lifetime))
	require.NoError(t, err, "токен чужим ключом не подписан")
	for method, query := range map[string]string{http.MethodPost: putQuery, http.MethodPatch: ownedQuery(anna.User.ID, id), http.MethodDelete: ownedQuery(anna.User.ID, id)} {
		name := strings.ToLower(method) + ".txt"
		body := fields(anna.User.ID, id, "взлом")
		status, got, h := r.write(t, method, query, "app-key", anna.Access, body)
		assert.Equal(t, reference(t, "write_denied_"+name), headed(status, h, got), "отказ токену входа не равен эталону Supabase: %s", method)
		for token, why := range map[string]string{"": "без токена", "app-key": "ключ приложения"} {
			status, got, h = r.write(t, method, query, "app-key", token, body)
			assert.Equal(t, reference(t, "write_anonymous_"+name), headed(status, h, got), "отказ не равен эталону Supabase: %s, %s", method, why)
		}
		for token, why := range map[string]string{spoil(anna.Access): "испорченный токен", forged: "чужой ключ подписи"} {
			status, got, _ = r.write(t, method, query, "app-key", token, body)
			assert.Equal(t, answer(http.StatusUnauthorized, jwtInvalid), answer(status, got), "негодный токен не отклонён: %s, %s", method, why)
		}
		status, got, _ = r.write(t, method, query, "", "service-key", body)
		assert.Equal(t, http.StatusUnauthorized, status, "запись без ключа проекта принята: %s", method)
		assert.Contains(t, got, "No API key found in request", "отказ без ключа проекта изменился: %s", method)
	}
	kept, ok := r.saved(t, anna.User.ID, id)
	assert.True(t, ok && kept.note == "почта", "прямая запись изменила запись")
	assert.Equal(t, 1, r.count(t), "прямая запись добавила строку")
}

func TestWriteSyntax(t *testing.T) {
	r := start(t)
	anna := r.user(t)
	id := r.put(t, anna.User.ID, "почта")
	owner := anna.User.ID
	for name, c := range map[string]struct {
		method, query string
		body          any
	}{
		"изменение без владельца":   {http.MethodPatch, "?id=eq." + id, fields(owner, id, "x")},
		"удаление без записи":       {http.MethodDelete, "?user_id=eq." + owner, nil},
		"сравнение не равенством":   {http.MethodDelete, "?user_id=eq." + owner + "&id=gt." + id, nil},
		"владелец не uuid":          {http.MethodPatch, "?user_id=eq.abc&id=eq." + id, fields(owner, id, "x")},
		"сохранение с id не uuid":   {http.MethodPost, putQuery, fields(owner, "abc", "x")},
		"сохранение без владельца":  {http.MethodPost, putQuery, map[string]string{"id": id, "secret": "x"}},
		"сохранение не объектом":    {http.MethodPost, putQuery, "запись"},
		"изменение с телом не JSON": {http.MethodPatch, ownedQuery(owner, id), "x"},
	} {
		assert.Equal(t, answer(http.StatusBadRequest, syntax), r.service(t, c.method, c.query, c.body), "негодный запрос принят: %s", name)
	}
	kept, ok := r.saved(t, owner, id)
	assert.True(t, ok && kept.note == "почта", "негодный запрос изменил запись")
	assert.Equal(t, 1, r.count(t), "негодный запрос добавил или удалил строку")
}

func TestWriteMissingOwner(t *testing.T) {
	r := start(t)
	owner, id := uuid.New().String(), uuid.New().String()
	status, body, h := r.write(t, http.MethodPost, putQuery, "service-key", "service-key", fields(owner, id, "почта"))
	assert.Equal(t, http.StatusConflict, status, "запись без владельца не получила отказ конфликтом: %s", body)
	assert.Equal(t, "PostgREST; error=23503", h.Get("Proxy-Status"), "обработчик не узнает отказ без владельца")
}
