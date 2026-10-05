package account

import "errors"

var (
	// ErrExists сообщает, что email уже зарегистрирован.
	ErrExists = errors.New("пользователь уже зарегистрирован")
	// ErrCredentials сообщает, что email или пароль не подошли.
	ErrCredentials = errors.New("неверный email или пароль")
	// ErrLoggedOut сообщает, что на этом устройстве вход не выполнен.
	ErrLoggedOut = errors.New("сначала войдите: gophkeeper login")
	// ErrNoAnswer сообщает, что сервер недоступен или не ответил вовремя.
	ErrNoAnswer = errors.New("сервер не отвечает, попробуйте позже")
	// ErrServer сообщает о незнакомой ошибке сервера без её подробностей.
	ErrServer = errors.New("сервер вернул ошибку, попробуйте позже")
)

var codes = map[string]error{
	"user_already_exists":        ErrExists,
	"invalid_credentials":        ErrCredentials,
	"refresh_token_not_found":    ErrLoggedOut,
	"refresh_token_already_used": ErrLoggedOut,
	"session_expired":            ErrLoggedOut,
	"session_not_found":          ErrLoggedOut,
}

func failure(code string) error {
	if err, ok := codes[code]; ok {
		return err
	}
	return ErrServer
}
