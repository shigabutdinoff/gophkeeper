package server

import "github.com/go-chi/chi/v5"

func (a *auth) writeRoutes(ops chi.Router, s Settings) {
	// @Summary Сохранение записи
	// @Description Создаёт запись или заменяет note и secret существующей. Только со служебным ключом. Как PostgREST в Supabase.
	// @Tags records
	// @Accept json
	// @Produce json
	// @Param record body stored true "Запись с id и user_id"
	// @Success 200 {array} object{user_id=string} "Запись заменена"
	// @Success 201 {array} object{user_id=string} "Запись создана"
	// @Failure 400 {object} object{code=string,message=string} "Неверное тело"
	// @Failure 401 {object} object{code=string,message=string} "Нет служебного ключа"
	// @Failure 403 {object} object{code=string,message=string} "Токен пользователя вместо служебного ключа"
	// @Failure 409 {object} object{code=string,message=string} "Владельца нет на сервере"
	// @Security ProjectKey && Bearer
	// @Router /rest/v1/records [post]
	ops.Post("/rest/v1/records", a.write(s))
	// @Summary Изменение записи
	// @Description Меняет note и secret записи владельца. Только со служебным ключом. Как PostgREST в Supabase.
	// @Tags records
	// @Accept json
	// @Produce json
	// @Param user_id query string true "Владелец в виде eq.<uuid>"
	// @Param id query string true "Запись в виде eq.<uuid>"
	// @Param record body object{note=string,secret=string} true "Новые note и secret"
	// @Success 200 {array} object{user_id=string} "Изменённые записи, пустой массив если записи нет"
	// @Failure 400 {object} object{code=string,message=string} "Неверный фильтр или тело"
	// @Failure 401 {object} object{code=string,message=string} "Нет служебного ключа"
	// @Failure 403 {object} object{code=string,message=string} "Токен пользователя вместо служебного ключа"
	// @Security ProjectKey && Bearer
	// @Router /rest/v1/records [patch]
	ops.Patch("/rest/v1/records", a.write(s))
	// @Summary Удаление записи
	// @Description Удаляет запись владельца. Только со служебным ключом. Как PostgREST в Supabase.
	// @Tags records
	// @Produce json
	// @Param user_id query string true "Владелец в виде eq.<uuid>"
	// @Param id query string true "Запись в виде eq.<uuid>"
	// @Success 200 {array} object{user_id=string} "Удалённые записи, пустой массив если записи нет"
	// @Failure 400 {object} object{code=string,message=string} "Неверный фильтр"
	// @Failure 401 {object} object{code=string,message=string} "Нет служебного ключа"
	// @Failure 403 {object} object{code=string,message=string} "Токен пользователя вместо служебного ключа"
	// @Security ProjectKey && Bearer
	// @Router /rest/v1/records [delete]
	ops.Delete("/rest/v1/records", a.write(s))
}
