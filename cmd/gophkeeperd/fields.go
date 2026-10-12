package main

import "github.com/shigabutdinoff/gophkeeper/internal/config"

type settings struct {
	Queue          string `flag:",required" desc:"адрес очереди изменений"`
	Server         string `flag:",required" desc:"адрес сервера"`
	ServerKey      string `flag:",required" desc:"ключ доступа к серверу"`
	AccessSeed     string `flag:",required" desc:"ключ выдачи доступа к очереди"`
	Region         string `flag:",required" desc:"регион хранилища файлов сервера"`
	AppKey         string `flag:",required" desc:"ключ приложения на сервере"`
	QueueAccount   string `desc:"ключ учётной записи очереди"`
	QueueCA        string `desc:"PEM сертификата очереди"`
	QueueUser      string `desc:"имя пользователя очереди"`
	QueuePassword  string `desc:"пароль пользователя очереди"`
	QueueCredsFile string `desc:"файл доступа к очереди"`
}

func (s settings) queue() config.Settings {
	return config.Settings{Queue: s.Queue, QueueUser: s.QueueUser, QueuePass: s.QueuePassword,
		QueueCreds: s.QueueCredsFile, QueueCA: s.QueueCA}
}
