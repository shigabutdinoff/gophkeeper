# GophKeeper

Менеджер паролей GophKeeper хранит логины и пароли, тексты, файлы и данные
банковских карт и отдаёт их в командной строке на любом вашем устройстве.

Данные шифруются на устройстве: сервер и даже его владелец видят их только
зашифрованными, открыты лишь пометки для поиска.

Клиент работает в macOS, Linux и Windows на amd64 и arm64.

## Установка

Скачайте файл для своей системы со страницы [Releases](https://github.com/shigabutdinoff/gophkeeper/releases).

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

## Использование

```sh
gophkeeper                         # список команд
gophkeeper <команда> --help        # справка по команде
gophkeeper --version               # версия, номер, дата и коммит сборки
gophkeeper completion <оболочка>   # скрипт автодополнения
gophkeeper register                # регистрация по email и паролю
gophkeeper login                   # вход по email и паролю
gophkeeper logout                  # выход на этом устройстве
```

Адрес сервера клиент берёт из первого заданного источника:

1. переменная `GOPHKEEPER_SERVER`;
2. флаг `--server`;
3. `server` в файле `~/.config/gophkeeper/config.yaml`, который пишет
   песочница.

Ключ приложения клиент берёт только из `app_key` в этом файле, поэтому
адрес из переменной или флага должен совпадать с `server` в нём. Пароль
вводится только с клавиатуры. Сервер получает не пароль, а выведенный из
него ключ входа, поэтому не знает ни пароля, ни ключа к данным.

Регистрация и вход запоминают вход на этом устройстве: ключ к данным и
токен обновления лежат в системном хранилище секретов (связка ключей
macOS, Credential Manager Windows, Secret Service Linux). Если хранилища
нет, пароль будет запрашиваться при каждой команде с данными. `logout`
завершает только сессию этого устройства.

## Разработка

<details>
<summary>Supabase, облачная очередь и локальная песочница</summary>

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

Нужен только Go. Запускайте песочницу из корня исходников от того же
пользователя и в той же среде, что и клиент: песочница из WSL не пишет настройки
для клиента в Windows. Очередь слушает `127.0.0.1:54222`, её данные,
сертификаты и журнал `sandbox.log` лежат в `~/.config/gophkeeper/sandbox`.

```sh
export SUPABASE_URL=https://<проект>.supabase.co SUPABASE_ANON_KEY=<ключ>
go run ./cmd/sandbox up      # запуск в фоне; первый выпускает сертификаты и секреты, пишет файл настроек
go run ./cmd/sandbox down    # остановка; очередь, сертификаты и файл настроек остаются
go run ./cmd/sandbox reset   # сброс: останавливает песочницу, удаляет очередь, сертификаты, секреты и файл настроек песочницы
```

</details>
