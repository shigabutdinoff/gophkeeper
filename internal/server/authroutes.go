package server

import "github.com/go-chi/chi/v5"

func (a *auth) authRoutes(ops chi.Router) {
	// @Summary Регистрация пользователя
	// @Description Создаёт учётную запись и сразу выдаёт сессию. Повторяет метод /signup Supabase Auth
	// @Description https://github.com/supabase/auth/blob/ce9a8eee0cc042be8c7a42981a7ddae631e41d91/openapi.yaml (лицензия MIT).
	// @Tags auth
	// @Accept json
	// @Produce json
	// @Param credentials body credentials true "Почта и пароль"
	// @Success 200 {object} issued
	// @Failure 400 {object} object{code=int,error_code=string,msg=string} "validation_failed"
	// @Failure 422 {object} object{code=int,error_code=string,msg=string} "user_already_exists"
	// @Security ProjectKey
	// @Router /auth/v1/signup [post]
	ops.Post("/auth/v1/signup", a.signup)
	// @Summary Вход и продление сессии
	// @Description Выдаёт токены по паролю (grant_type=password, тело с email и password) или по токену
	// @Description продления (grant_type=refresh_token, тело с refresh_token). Повторяет метод /token Supabase Auth
	// @Description https://github.com/supabase/auth/blob/ce9a8eee0cc042be8c7a42981a7ddae631e41d91/openapi.yaml (лицензия MIT).
	// @Tags auth
	// @Accept json
	// @Produce json
	// @Param grant_type query string true "Способ входа" Enums(password, refresh_token)
	// @Param body body object{email=string,password=string,refresh_token=string} true "Почта и пароль или токен продления"
	// @Success 200 {object} issued
	// @Failure 400 {object} object{code=int,error_code=string,msg=string} "invalid_credentials, refresh_token_not_found или validation_failed"
	// @Security ProjectKey
	// @Router /auth/v1/token [post]
	ops.Post("/auth/v1/token", a.token)
	// @Summary Выход на этом устройстве
	// @Description Завершает сессию токена входа. Повторяет метод /logout Supabase Auth
	// @Description https://github.com/supabase/auth/blob/ce9a8eee0cc042be8c7a42981a7ddae631e41d91/openapi.yaml (лицензия MIT).
	// @Tags auth
	// @Produce json
	// @Param scope query string true "Какие сессии завершить" Enums(local)
	// @Success 204
	// @Failure 400 {object} object{code=int,error_code=string,msg=string} "validation_failed"
	// @Security ProjectKey && Bearer
	// @Router /auth/v1/logout [post]
	ops.Post("/auth/v1/logout", a.logout)
}
