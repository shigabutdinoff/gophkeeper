package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (a *auth) fileRoutes(r chi.Router, s Settings) {
	file := http.MaxBytesHandler(a.file(s), objectLimit)
	// @Summary Сохранение файла
	// @Description Сохраняет файл записи до 50 МиБ. Только со служебным ключом в X-Amz-Security-Token. Как S3 в Supabase Storage,
	// @Description подпись SigV4 клиент S3 ставит сам, сервер её не проверяет.
	// @Tags files
	// @Accept application/octet-stream
	// @Produce plain
	// @Param owner path string true "Владелец записи"
	// @Param id path string true "Файл"
	// @Param file body string true "Содержимое файла"
	// @Success 200 "Файл сохранён, ETag в заголовке"
	// @Failure 403 {string} string "Ошибка S3 в XML, нет служебного ключа"
	// @Failure 413 {string} string "Ошибка S3 в XML, файл больше предела"
	// @Security S3Token
	// @Router /storage/v1/s3/files/{owner}/{id} [put]
	r.Method(http.MethodPut, filesPath, file)
	// @Summary Выдача файла
	// @Description Отдаёт файл владельцу токена входа или со служебным ключом в X-Amz-Security-Token. Как S3 в Supabase Storage.
	// @Tags files
	// @Produce application/octet-stream,plain
	// @Param owner path string true "Владелец записи"
	// @Param id path string true "Файл"
	// @Success 200 {file} binary
	// @Failure 403 {string} string "Ошибка S3 в XML, токен не подходит"
	// @Failure 404 {string} string "Ошибка S3 в XML, файла нет или он чужой"
	// @Security S3Token
	// @Router /storage/v1/s3/files/{owner}/{id} [get]
	r.Method(http.MethodGet, filesPath, file)
	// @Summary Удаление файла
	// @Description Удаляет файл, отсутствующий файл тоже считается удалённым. Только со служебным ключом. Как S3 в Supabase Storage.
	// @Tags files
	// @Produce plain
	// @Param owner path string true "Владелец записи"
	// @Param id path string true "Файл"
	// @Success 204
	// @Failure 403 {string} string "Ошибка S3 в XML, нет служебного ключа"
	// @Security S3Token
	// @Router /storage/v1/s3/files/{owner}/{id} [delete]
	r.Method(http.MethodDelete, filesPath, file)
}
