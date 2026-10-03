// Command gophkeeper запускает клиент менеджера паролей GophKeeper.
//
// Клиент понимает такие команды.
//
//	gophkeeper completion <оболочка>  скрипт автодополнения
//	gophkeeper help [команда]         справка по командам
//
// Флаг version печатает версию, номер, дату и коммит сборки. Автодополнение
// есть для bash, zsh, fish и powershell. Версию, номер сборки, дату и коммит
// подставляет GoReleaser при сборке.
package main
