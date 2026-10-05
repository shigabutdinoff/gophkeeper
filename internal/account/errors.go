package account

import (
	"encoding/json"
	"errors"
	"io"
)

var (
	// ErrExists сообщает, что email уже зарегистрирован.
	ErrExists = errors.New("пользователь уже зарегистрирован")
	// ErrNoAnswer сообщает, что сервер недоступен или не ответил вовремя.
	ErrNoAnswer = errors.New("сервер не отвечает, попробуйте позже")
	// ErrServer сообщает о незнакомой ошибке сервера без её подробностей.
	ErrServer = errors.New("сервер вернул ошибку, попробуйте позже")
)

func failure(body io.Reader) error {
	var reply struct {
		Code string `json:"error_code"`
	}
	if json.NewDecoder(io.LimitReader(body, 1<<16)).Decode(&reply) == nil && reply.Code == "user_already_exists" {
		return ErrExists
	}
	return ErrServer
}
