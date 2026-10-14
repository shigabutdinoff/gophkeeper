package account

import "errors"

var (
	// ErrExists сообщает, что email уже зарегистрирован.
	ErrExists = errors.New("пользователь уже зарегистрирован")
	// ErrCredentials сообщает, что email или пароль не подошли.
	ErrCredentials = errors.New("неверный email или пароль")
	// ErrLoggedOut сообщает, что на этом устройстве вход не выполнен.
	ErrLoggedOut = errors.New("сначала войдите в GophKeeper: gophkeeper login")
	// ErrNoAnswer сообщает, что сервер недоступен или не ответил вовремя.
	ErrNoAnswer = errors.New("сервер не отвечает, попробуйте позже")
	// ErrServer сообщает о незнакомой ошибке сервера без её подробностей.
	ErrServer = errors.New("сервер вернул ошибку, попробуйте позже")
	// ErrUnavailable сообщает, что соединения с сервером нет.
	ErrUnavailable = errors.New("сервер недоступен, попробуйте позже")
	// ErrUntrusted сообщает, что сертификат сервера не прошёл проверку.
	ErrUntrusted = errors.New("сертификат сервера не доверенный")
	// ErrNotFound сообщает, что записи нет у владельца.
	ErrNotFound = errors.New("запись не найдена")
)

func failure(code string) error {
	switch code {
	case "user_already_exists":
		return ErrExists
	case "invalid_credentials":
		return ErrCredentials
	case "refresh_token_not_found", "refresh_token_already_used", "session_expired", "session_not_found":
		return ErrLoggedOut
	}
	return ErrServer
}
