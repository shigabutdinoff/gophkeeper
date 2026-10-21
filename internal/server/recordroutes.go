package server

import "github.com/go-chi/chi/v5"

func (a *auth) recordRoutes(ops chi.Router) {
	// @Summary Записи владельца токена
	// @Description Отдаёт записи владельца токена входа по возрастанию id, не больше 1000 за запрос. Как PostgREST в Supabase.
	// @Tags records
	// @Produce json
	// @Param id query string false "Фильтр по id в виде eq.<uuid> или gt.<uuid>"
	// @Param note query string false "Поиск по пометке без учёта регистра в виде ilike.*<подстрока>*"
	// @Param limit query int false "Сколько записей отдать, не больше 1000"
	// @Success 200 {array} entry
	// @Failure 400 {object} object{code=string,message=string} "Неверный фильтр"
	// @Failure 401 {object} object{code=string,message=string} "Токен входа не подходит"
	// @Security ProjectKey && Bearer
	// @Router /rest/v1/records [get]
	ops.Get("/rest/v1/records", a.records)
}
