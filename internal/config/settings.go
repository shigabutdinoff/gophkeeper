package config

// Settings содержит настройки клиента для связи с сервером и очередью
// изменений.
type Settings struct {
	// Server задаёт адрес сервера.
	Server string `yaml:"server"`
	// AppKey задаёт открытый ключ приложения на сервере.
	AppKey string `yaml:"app_key"`
	// Queue задаёт адрес очереди изменений.
	Queue string `yaml:"queue"`
	// QueueUser задаёт имя пользователя очереди.
	QueueUser string `yaml:"queue_user,omitempty"`
	// QueuePass задаёт пароль пользователя очереди.
	QueuePass string `yaml:"queue_password,omitempty"`
	// QueueCreds задаёт путь к файлу доступа очереди.
	QueueCreds string `yaml:"queue_creds_file,omitempty"`
	// QueueInbox задаёт префикс тем для ответов очереди.
	QueueInbox string `yaml:"queue_inbox"`
	// QueueCA задаёт PEM удостоверяющего центра очереди.
	QueueCA string `yaml:"queue_ca,omitempty"`
}
