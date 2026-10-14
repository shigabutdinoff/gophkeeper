# GophKeeper

Менеджер паролей GophKeeper хранит логины и пароли, тексты, файлы и данные
банковских карт и отдаёт их в командной строке на любом вашем устройстве.
Данные шифруются на устройстве: сервер и даже его владелец видят их только
зашифрованными, открыты лишь пометки для поиска.

Клиент работает в macOS, Linux и Windows на amd64 и arm64. Он подключается к
своему серверу gophkeeperd или к облачным Supabase и Synadia.

## Документация

```sh
gophkeeper                              # список команд
gophkeeper <команда> --help             # справка по команде
gophkeeper --version                    # версия, номер, дата и коммит сборки
gophkeeper completion <оболочка>        # скрипт автодополнения
gophkeeper register                     # регистрация по email и паролю
gophkeeper login                        # вход по email и паролю
gophkeeper logout                       # выход на этом устройстве
gophkeeper add --login <логин>          # логин и пароль, флаг --note добавляет пометку
gophkeeper add --type text              # текст из stdin до конца ввода
gophkeeper add --type card              # номер, срок, имя и код банковской карты
gophkeeper add --type file              # файл до 50 МБ, выбирается в форме
gophkeeper list                         # тип, пометка и идентификатор каждой записи
gophkeeper list <идентификатор>         # поля и пометка записи
gophkeeper get <идентификатор> <путь>   # файл записи на диск
gophkeeper edit <идентификатор>         # новый пароль, флаги --login и --note меняют остальное
gophkeeper delete <идентификатор>       # удаление записи
```

## Установка

Скачайте файл для своей системы со [страницы выпусков][].

### macOS

```sh
xattr -d com.apple.quarantine gophkeeper-*-darwin-*
mv gophkeeper-*-darwin-* gophkeeper
chmod +x gophkeeper
sudo mv gophkeeper /usr/local/bin/
```

### Linux

```sh
mv gophkeeper-*-linux-* gophkeeper
chmod +x gophkeeper
sudo mv gophkeeper /usr/local/bin/
```

### Windows

Переименуйте файл в `gophkeeper.exe` и положите в каталог из `Path`.

### Из исходников

Нужны Go, git и make.

```sh
make build     # ./gophkeeper для текущей системы
make release   # файлы для всех систем в dist/
```

## Сервер

### Песочница

Песочница поднимает сервер на этом компьютере и записывает настройки клиента,
после неё сразу работают `register` и `login`. Команды вызываются из корня
репозитория, для `--own` нужен запущенный Docker.

```sh
go run ./cmd/sandbox up --own   # свой сервер, Postgres в Docker и очередь изменений
go run ./cmd/sandbox up         # очередь изменений здесь, сервер в облачном Supabase
go run ./cmd/sandbox cloud      # облачные Supabase и очередь Synadia
go run ./cmd/sandbox down       # остановка с сохранением данных
go run ./cmd/sandbox reset      # остановка с удалением данных и настроек
```

### Свой сервер

На странице выпусков лежит только клиент, сервер собирается из исходников:

```sh
go build ./cmd/gophkeeperd
./gophkeeperd server --help     # приём запросов клиентов по TLS
./gophkeeperd worker --help     # применение изменений из очереди к серверу
```

[страницы выпусков]: https://github.com/shigabutdinoff/gophkeeper/releases
