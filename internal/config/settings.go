package config

import "errors"

// ErrInsecureServer сообщает, что адрес сервера не начинается с https.
var ErrInsecureServer = errors.New("сервер должен работать по защищённому соединению (https)")

// Settings содержит настройки клиента для связи с сервером и очередью
// изменений.
type Settings struct {
	// Server задаёт адрес сервера.
	Server string `yaml:"server"`
	// AppKey задаёт открытый ключ приложения на сервере.
	AppKey string `yaml:"app_key"`
	// Region задаёт регион хранилища файлов сервера.
	Region string `yaml:"region,omitempty"`
	// Queue задаёт адрес очереди изменений.
	Queue string `yaml:"queue"`
	// QueueLeaf задаёт адрес очереди изменений для очереди устройства.
	QueueLeaf string `yaml:"queue_leaf,omitempty"`
	// QueueUser задаёт имя пользователя очереди.
	QueueUser string `yaml:"queue_user,omitempty"`
	// QueuePass задаёт пароль пользователя очереди.
	QueuePass string `yaml:"queue_password,omitempty"`
	// QueueCreds задаёт путь к файлу доступа очереди.
	QueueCreds string `yaml:"queue_creds_file,omitempty"`
	// QueueCA задаёт PEM удостоверяющего центра очереди.
	QueueCA string `yaml:"queue_ca,omitempty"`
	// ServerCert задаёт путь к файлу сертификата сервера.
	ServerCert string `yaml:"server_cert,omitempty"`
}
