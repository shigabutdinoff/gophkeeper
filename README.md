# GophKeeper

Менеджер паролей GophKeeper

GophKeeper хранит логины и пароли, тексты, файлы и данные банковских карт и
отдаёт их на любом вашем устройстве. Данные шифруются на устройстве: сервер и
даже его владелец видят их только зашифрованными, открыты лишь пометки для
поиска.

Клиент работает в macOS, Linux и Windows на amd64 и arm64.

## Установка

Скачайте файл для своей системы со страницы [Releases](https://github.com/shigabutdinoff/gophkeeper/releases).

### macOS

```sh
xattr -d com.apple.quarantine gophkeeper-27.0-*-darwin-*
mv gophkeeper-27.0-*-darwin-* gophkeeper
chmod +x gophkeeper
sudo mv gophkeeper /usr/local/bin/
```

### Linux

```sh
mv gophkeeper-27.0-*-linux-* gophkeeper
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

## Использование

```sh
gophkeeper                         # список команд
gophkeeper <команда> --help        # справка по команде
gophkeeper --version               # версия, номер, дата и коммит сборки
gophkeeper completion <оболочка>   # скрипт автодополнения
```

## Разработка

В Supabase создайте проект и выключите подтверждение email: откройте
Authentication, затем Sign In / Providers, снимите Confirm email и нажмите
Save changes. Адрес проекта показан на его главной странице под названием,
открытый ключ `anon` лежит в Project Settings, затем API Keys, на вкладке
Legacy anon, service_role API keys.

### Облачная очередь

В Synadia Cloud заведите двух пользователей и скачайте их `.creds`: `admin`
без ограничений и `client`, которому разрешены только публикация в
`changes.>` и подписка на `_INBOX.pub.>`.

```sh
export SUPABASE_URL=https://<проект>.supabase.co SUPABASE_ANON_KEY=<ключ>
export SYNADIA_ADMIN_CREDS=~/admin.creds SYNADIA_CLIENT_CREDS=~/client.creds
go run ./cmd/sandbox cloud
```

### Локальная песочница

Нужен Docker с Compose 2.30 или новее, в Linux Docker Engine 28.3.3 или
новее. Запускайте песочницу из корня исходников без `sudo`, от того же
пользователя и в той же среде, что и клиент: песочница из WSL не пишет настройки
для клиента в Windows.

```sh
export SUPABASE_URL=https://<проект>.supabase.co SUPABASE_ANON_KEY=<ключ>
docker compose up -d --wait   # запуск; первый скачивает модули Go, выпускает сертификаты и секреты
docker compose logs init      # что запуск сделал с файлом настроек
docker compose stop           # остановка; данные, очередь и сертификаты остаются, файл настроек песочницы удаляется до запуска
docker compose down -v        # сброс: удаляет очередь, сертификаты, секреты, кэш Go и файл настроек песочницы
```

### Проверка в живой песочнице

```sh
go test -tags e2e ./cmd/sandbox/e2e
```
