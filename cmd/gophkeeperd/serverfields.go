package main

type serverSettings struct {
	Address   string `flag:",required" desc:"адрес приёма соединений"`
	Cert      string `desc:"файл сертификата сервера"`
	Key       string `desc:"файл ключа сервера"`
	TokenKey  string `flag:",required" desc:"файл ключа подписи токенов"`
	AppKey    string `flag:",required" desc:"ключ приложения на сервере"`
	ServerKey string `flag:",required" desc:"ключ доступа к серверу"`
	Database  string `flag:",required" desc:"адрес базы данных"`
	queueSettings
	FilesLimit string `flag:",required" desc:"предел объёма корзины файлов"`
}
