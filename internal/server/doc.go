// Package server обслуживает собственный сервер GophKeeper по защищённому
// соединению и пускает к операциям только запросы с ключом проекта.
// Он регистрирует пользователей, выполняет вход, продлевает сессию и
// завершает её на одном устройстве с теми же ответами, что Supabase.
// Учётные записи, сессии и записи пользователей хранит в Postgres, отдаёт
// владельцу токена входа только его записи и открытые ключи для проверки
// своих токенов входа. Сохранение и удаление записей принимает только со
// служебным ключом в пределах владельца, указанного в запросе.
// Файлы записей хранит в своей корзине очереди и отвечает на запросы S3 так
// же, как Supabase Storage: сохраняет и удаляет файл только со служебным
// ключом, а выдаёт владельцу токена входа или служебному ключу.
// Описание своего API в формате Swagger 2.0 отдаёт без ключа проекта по
// адресу /swagger/doc.json.
package server

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --dir . --generalInfo doc.go --parseFuncBody --outputTypes json,yaml --output ../../api

// @title GophKeeper
// @version 27.0
// @description Собственный сервер GophKeeper. Отвечает на те же адреса и в том же формате, что Supabase Auth,
// @description PostgREST и S3 в Supabase Storage, поэтому клиент работает и с ним, и с облачным Supabase.
// @license.name MIT
// @schemes https
// @securityDefinitions.apikey ProjectKey
// @in header
// @name apikey
// @description Ключ проекта, публичный или служебный.
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Bearer и токен входа, для сохранения и удаления записей Bearer и служебный ключ.
// @securityDefinitions.apikey S3Token
// @in header
// @name X-Amz-Security-Token
// @description Токен входа или служебный ключ.
