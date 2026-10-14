package queuetest

import (
	"math/rand/v2"
	"net"
	"strconv"
)

// FreePort возвращает свободный порт на 127.0.0.1 ниже диапазона временных
// портов системы. Такой порт не займёт чужое исходящее соединение, пока
// сервер на нём ещё не запущен.
func FreePort() (int, error) {
	var err error
	for range 100 {
		port := 20000 + rand.IntN(12000)
		l, lerr := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if lerr == nil {
			return port, l.Close()
		}
		err = lerr
	}
	return 0, err
}
