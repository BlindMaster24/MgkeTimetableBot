# МГКЦТ Бот расписаний

[English version](README.en.md)

[![CI](https://github.com/BlindMaster24/MgkeTimetableBot/actions/workflows/go-ci.yml/badge.svg?branch=main)](https://github.com/BlindMaster24/MgkeTimetableBot/actions/workflows/go-ci.yml)
[![Go 1.27.1](https://img.shields.io/badge/go-1.27.1-00ADD8)](https://go.dev/dl/)

## Описание

МГКЦТ Бот расписаний — Telegram-бот для удобного просмотра расписания Минского государственного колледжа цифровых технологий.

Это полный порт старого TypeScript-бота на Go: тот же набор команд, меню, кнопок и текстов, без VK / Viber / Алисы — только Telegram и HTTP API. Паритет поверхности (команды, callback-префиксы, подписи кнопок) проверяется автоматически — см. [Паритет с TypeScript-ботом](#паритет-с-typescript-ботом).

Возможности:

- просмотр расписания на день и на неделю для группы и преподавателя, с учётом числителя/знаменателя;
- расписание звонков с автообновлением с сайта колледжа;
- вывод расписания картинкой (PNG, чистый Go, без CGO);
- экспорт в ICS и двусторонняя связка с Google Calendar;
- архив расписания в SQLite + уведомления об изменениях;
- настраиваемые меню: формат вывода, отображение, оповещения, сравнение, алиасы;
- REST API на gin.

## Стек

- [telego](https://github.com/mymmrac/telego) — Telegram Bot API (long polling)
- [gin](https://github.com/gin-gonic/gin) — HTTP-сервер и REST API
- [goquery](https://github.com/PuerkitoBio/goquery) — парсинг HTML сайта
- [zerolog](https://github.com/rs/zerolog) + [lumberjack](https://github.com/natefinch/lumberjack) — логирование и ротация
- [go-i18n](https://github.com/nicksnyder/go-i18n) — интернационализация (`internal/i18n/locales/ru.json`)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — SQLite чистым Go, без CGO
- [fogleman/gg](https://github.com/fogleman/gg) — рендеринг расписания в PNG
- [robfig/cron](https://github.com/robfig/cron) — планировщик уведомлений
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) — конфигурация
- ICS генерируется вручную в `internal/calendar`, внешней библиотеки нет

## Требования

- **Минимальная версия Go — 1.27.1** (см. [CI](https://github.com/BlindMaster24/MgkeTimetableBot/actions/workflows/go-ci.yml)).
- CGO не нужен: SQLite и рендеринг картинок работают на чистом Go.
- Для работы нужен только записываемый каталог для `cache/` и файлов базы.

Проверить версию:

```bash
go version   # ожидается go1.27.1 или новее
```

## Установка и запуск

1. Клонируйте репозиторий:

    ```bash
    git clone https://github.com/BlindMaster24/MgkeTimetableBot.git
    cd MgkeTimetableBot
    ```

2. Создайте конфиг из шаблона и заполните его:

    ```bash
    cp configs/config.example.yaml configs/config.yaml
    # укажите telegram.token, telegram.admin_ids, http.port, ключи Google и т.д.
    ```

3. Установите зависимости:

    ```bash
    go mod download
    ```

4. Запустите бота:

    ```bash
    go run ./cmd/bot/ -config configs/config.yaml
    ```

5. Или соберите бинарник и запустите его:

    ```bash
    go build -o bot ./cmd/bot/
    ./bot -config configs/config.yaml
    ```

Путь к конфигу можно задать и переменной окружения `CONFIG_PATH` — флаг `-config` имеет приоритет. Без обоих значений используется `configs/config.yaml`.

Бот в одном процессе поднимает Telegram long polling, HTTP-сервер (API + OAuth-callback Google) и горутину парсера.

## Конфигурация

Все настройки — в `configs/config.yaml` (шаблон: `configs/config.example.yaml`).

| Секция | Назначение |
|--------|------------|
| `db_path` | Путь к SQLite с архивом расписания (по умолчанию `./sqlite3.db`) |
| `chat_db_path` | Путь к SQLite с чатами, алиасами, подписками, аккаунтами Google (по умолчанию `./bot_chats.db`) |
| `cache_dir` | Каталог файлового кэша расписания (по умолчанию `./cache/rasp`) |
| `logging` | Уровень, файл лога, параметры ротации |
| `http` | Порт HTTP-сервера (API и Google OAuth) |
| `telegram` | Токен бота, ID администраторов, флаг `noticer` |
| `api` | Базовый путь REST API |
| `google` | OAuth-клиент и service account для Google Calendar |
| `calendar.ics.enabled` | Включить экспорт ICS и кнопку в меню |
| `accept` | Что показывать в расписании (аудитории, приватные записи) |
| `parser` | Интервал опроса, источники, расписание звонков, прокси |
| `timetable` | Резервное расписание звонков: `weekdays`, `saturday` (используется, когда сайт недоступен) |
| `health` | Пороги алертов по здоровью: парсер, синхронизация календарей, ошибки API |
| `encrypt_key` | Ключ шифрования (для `createApiKey` / `decryptKey`) |

Кэш расписания лежит в `cache/rasp/` в виде JSON, архив — в SQLite (см. `db_path` и `migrations/001_init.up.sql`).

### Переменные окружения

Любое поле конфига можно переопределить переменной окружения — это удобно в Docker и CI, где секреты не хочется класть в файл.

- имя по умолчанию — префикс `MGKE_` плюс путь поля через `_`: `MGKE_HTTP_PORT`, `MGKE_DB_PATH`, `MGKE_PARSER_ENABLED`, `MGKE_TELEGRAM_TOKEN`, `MGKE_GOOGLE_SERVICE_ACCOUNT_PRIVATE_KEY`;
- у части полей есть короткие исторические алиасы: `DB_PATH`, `LOG_LEVEL`, `HTTP_PORT`, `TG_TOKEN`, `ENCRYPT_KEY`. Если заданы оба имени, побеждает `MGKE_*`;
- приоритет: аргумент `-config` → переменные окружения → файл конфига. Исключение — `CONFIG_PATH`: он только задаёт путь к файлу и используется, когда флаг не передан;
- `MGKE_` перекрывает любое значение из YAML, пустая строка считается явным значением (например, `MGKE_ENCRYPT_KEY=`);
- списки и таблицы фиксированной длины — через запятую: `MGKE_TELEGRAM_ADMIN_IDS=1,2,3`, `MGKE_PARSER_ACTIVITY=8,20`;
- булевы значения понимают `true/false`, `1/0`, `yes/no`, `on/off`;
- сложные структуры (например, таблицы звонков `timetable.weekdays`) переменными не задаются — их место в YAML; если такую переменную всё же задать, бот упадёт с явной ошибкой вместо тихого игнорирования;
- некорректное значение (например, `MGKE_HTTP_PORT=abc`) — тоже ошибка запуска с указанием имени переменной.

```bash
CONFIG_PATH=configs/config.yaml \
MGKE_TELEGRAM_TOKEN=123:ABC \
MGKE_HTTP_PORT=8080 \
MGKE_PARSER_ENABLED=true \
MGKE_TELEGRAM_ADMIN_IDS=1,2,3 \
./bot
```

Полный список имён можно получить из кода: `config.EnvNames()` возвращает все поддерживаемые переменные.

### Парсер

- `parser.enabled` — включить фоновый парсинг;
- `parser.endpoints.*` — адреса страниц сайта: расписание групп, расписание преподавателей, список преподавателей, расписание звонков;
- `parser.update_interval.*` — интервалы опроса: обычный, в часы активности, после ошибки, для списка преподавателей, для звонков;
- `parser.alertable_ignore_filter` и `parser.lesson_index_if_empty` — какие пары считаются значимыми при уведомлениях об изменениях;
- `parser.calls.enabled`, `parser.calls.prefer_site`, `parser.calls.notify` — звонки: включены ли, что приоритетнее — сайт или ручная правка, уведомлять ли об изменениях;
- `parser.proxy` — HTTP(S)-прокси для запросов к сайту: `http://user:pass@host:port`. Если сайт колледжа не открывается напрямую (домен `.by` заблокирован или недоступен без VPN), укажите прокси или локальный адрес VPN-клиента — например `socks5`-порт Throne; при некорректном URL бот пишет предупреждение и продолжает работать напрямую.

Планировщик учитывает расписание занятий: днём (по умолчанию 9:00–17:00, ключ `parser.activity`) опрос идёт часто, в остальное время — с обычным интервалом, по воскресеньям активное окно отключено, а после ошибки бот возвращается к опросу через `parser.update_interval.error`. Список преподавателей (страницы `parser.endpoints.team`) обновляется раз в сутки, звонки — по своему интервалу.

Врезка расписания звонков обновляется автоматически со страницы сайта; при необходимости его можно править вручную из меню звонков.

#### Устойчивость к изменениям вёрстки

Парсер не привязан к жёсткой структуре страницы: имя группы или преподавателя он берёт из ближайшего заголовка (в любом теге — `h1`…`h6`, `caption`), колонки дней — из шапки таблицы с учётом `colspan`, а не из фиксированных номеров ячеек. Если вёрстка упростится до одной колонки на день, аудиторию парсер попробует вытащить из текста ячейки.

Каждый прогон собирает диагностику — по каким селекторам что нашлось. Если сайт поменяется и обязательный селектор перестанет совпадать, пустой или резко сократившийся результат **не перезапишет кэш**: бот продолжит отдавать расписание, а проблема появится в `/parserLogs` (админам) и в `GET /api/health` (`parser.layout`) и, после `health.parser_layout_failures` прогонов подряд, придёт алертом в Telegram.

Ключи `parser.v2.*` остались от старого TypeScript-бота и Go-версией не читаются — в шаблоне конфига они больше не приводятся.

## Команды бота

### Расписание и настройка

| Команда | Описание |
|---------|----------|
| `/start` | Начало работы, главное меню |
| `/help` | Справка |
| `/setup` | Первоначальная настройка: выбрать режим, группу или преподавателя |
| `/day` | Расписание на сегодня (стрелками можно листать дни) |
| `/week` | Расписание на неделю (с переключением недель — учебная неделя тоже) |
| `/calls` | Расписание звонков |
| `/group` | Расписание группы |
| `/teacher` | Расписание преподавателя |
| `/image` | Расписание картинкой |
| `/cabinet` | Поиск по кабинету |
| `/settings` | Меню настроек: точка входа в подменю ниже |
| `/formatter` | Меню выбора формата вывода расписания |
| `/view` | Меню отображения: скрывать прошедшие дни, показывать время загрузки, подсказки |
| `/notice` | Меню оповещений: изменения, новая неделя, звонки, ошибки парсера |
| `/diff` | Меню отображения изменений расписания |
| `/buttons` | Меню кнопок |
| `/subscriptions` | Меню подписок на чужие расписания |
| `/alias` | Алиасы: сохранить группу или преподавателя под своим названием |
| `/history` | История расписания из архива |
| `/archive` | Просмотр архивных дней |
| `/comparegroups` | Сравнение расписаний двух групп |
| `/groups`, `/teachers` | Список групп и преподавателей |
| `/groupweek`, `/groupimage`, `/teacherweek`, `/teacherimage` | Прямые ссылки на недельное расписание и картинку |
| `/ics` | Экспорт расписания в `.ics` (нужен `calendar.ics.enabled: true`) |
| `/google_calendar` | Google Calendar: список, добавление, права |
| `/about` | О боте |
| `/eula` | Пользовательское соглашение |
| `/cancel` | Отменить текущий диалог |
| `/api` | Информация об API и ключах |
| `/ping`, `/stats` | Проверка доступности и статистика |

Внутренние служебные команды меню (`/btn_toggle_text_*`, `/view_toggle_text_*`, `/notice_toggle_text_*`, `/diff_toggle_text_*`, `/formatter_select_text*`, `/settings_nav_*`, `/calls_settings_text_*`, `/setup_mode_text_*`, `/alias_action_text_*`, `/subs_action_text_*`, `/show_current_settings_text`) регистрируются декларативной таблицей меню и вводятся вручную не пользователем, а кнопками клавиатуры.

### Административные

Доступны только ID из `telegram.admin_ids` — в меню команд Telegram они показываются только им и с пометкой `[адм]`:

`/debug`, `/send`, `/trigger`, `/noticedebug`, `/archivestats`, `/forceparse`, `/resetcache`, `/flushcache`, `/buttons_reload`, `/parserLogs`, `/restart`, `/sql`, `/regexp`, `/vanish`, `/math`, `/dev`, `/createApiKey`, `/decryptKey`, `/requireNewButtons`, `/chat`, `/id`, `/error`, `/test`, `/endings`, `/subscriptions_test`, `/setgroup`, `/setteacher`, `/vychetkaDlyaBrovkiDSOnline`.

`/send` рассылает сообщение всем чатам с ограничением 25 сообщений в минуту, чтобы не упереться в лимиты Telegram.

## HTTP API

Сервер поднимается на `0.0.0.0:http.port`:

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/info` | Сведения о сервисе и версия |
| GET | `/api/groups` | Список групп |
| GET | `/api/teachers` | Список преподавателей |
| GET | `/api/group/:name` | Расписание группы |
| GET | `/api/teacher/:name` | Расписание преподавателя |
| GET | `/api/parser-health` | Счётчики кэша: попадания, промахи, время обновления |
| GET | `/api/health` | Метрики здоровья (парсер, календари, API) и активные алерты; 503, если есть алерты |

`google.url` (по умолчанию `/google/oauth`) — callback OAuth Google, сюда возвращается пользователь после авторизации.

## Метрики и здоровье

Счётчики собираются в память (`internal/health`) и отдаются в `GET /api/health`. Эндпоинт возвращает метрики и активные алерты, а при наличии алертов отвечает кодом `503` — так его можно сразу повесить на мониторинг.

Что отслеживается:

| Группа | Метрики | Алерты |
|--------|---------|--------|
| Парсер | число запусков и ошибок, серия неудач, время последней успешной загрузки (лаг), длительность цикла, какие селекторы перестали находить данные | серия неудач, устаревшие данные, поломка вёрстки сайта |
| Google Calendar | число запусков и ошибок, серия неудач, сколько дней синхронизировано | серия ошибок, давно не было успешной синхронизации |
| HTTP API | число запросов, ответы 5xx, число свежих ошибок, самый медленный запрос | всплеск ошибок 5xx |

Алерты уходят администраторам (`telegram.admin_ids`) в Telegram отдельным сообщением с подробностями и раз в `cooldown_minutes` повторяются, пока проблема не ушла; после восстановления приходит отдельное сообщение. Пороги настраиваются секцией `health`:

```yaml
health:
  disabled: false              # полностью выключить алерты
  check_minutes: 1             # как часто проверять
  cooldown_minutes: 30         # пауза между напоминаниями
  parser_stale_minutes: 15     # расписание давно не обновлялось
  parser_failures: 3           # подряд неудачных парсингов
  parser_layout_failures: 2    # подряд парсингов, где перестал находиться селектор
  calendar_stale_minutes: 360
  calendar_failures: 3
  api_errors: 20               # 5xx в окне
  api_window_minutes: 5
```

Все поля секции доступны и переменными окружения (`MGKE_HEALTH_PARSER_STALE_MINUTES` и т.д.).

## Уведомления

Планировщик (`internal/notification`) поднимается вместе с ботом и рассылает:

- изменения расписания на сегодня и на завтра;
- появление расписания на новую неделю;
- изменения расписания звонков;
- ошибки парсера.

События формируются парсером, складываются в кэш и разбираются после каждого цикла парсинга. Каждый тип уведомлений можно выключить в меню «Оповещения» (`/notice`), у каждого чата свои настройки.

## Google Calendar и ICS

- `docs/google-calendar.md` — настройка Google Cloud, OAuth, привязка аккаунта, права.
- Google-меню: `/google_calendar` или кнопка «📅 Google Calendar» в главном меню — список календарей, добавление, выдача и снятие прав.
- Синхронизация событийная: парсер после каждого цикла отдаёт только те дни, которые реально изменились, и бот обновляет ровно их — без перечитывания архива целиком. День перед записью очищается от старых событий, время пар берётся из расписания звонков.
- Отдельно работает выравнивание (reconcile): если календарь отстал (например, бот был выключен или началась новая неделя), бот добирает только недостающие дни — по одному разу, без повторной перезаписи уже синхронизированного.
- ICS: `/ics` при включённом `calendar.ics.enabled`.

## Развёртывание в Docker

```bash
docker compose up -d --build
```

`docker-compose.yml` собирает образ из `Dockerfile`, подкладывает конфиг `configs/config.yaml` и хранит состояние (`bot_chats.db`, `sqlite3.db`, `cache/`, логи) в именованном томе. Секреты передаются переменными окружения:

```yaml
services:
  bot:
    environment:
      MGKE_TELEGRAM_TOKEN: "123:ABC"
      MGKE_TELEGRAM_ADMIN_IDS: "1,2"
      MGKE_DB_PATH: /data/sqlite3.db
```

Что важно знать про образ:

- многостудийная сборка: бинарник собирается на `golang:1.27.1`, в рантайм-образ попадает только бинарник, конфиг-шаблон и сертификаты;
- процесс запускается от непривилегированного пользователя, CGO не нужен (SQLite и рендер картинок — чистый Go);
- встроенный `HEALTHCHECK` каждые 30 секунд обращается к `GET /api/health` и переводит контейнер в `unhealthy` при алертах;
- наружу отдаётся только HTTP-порт (`http.port`), Telegram работает через long polling — входящие порты больше не нужны;
- данные живут в томе `/data`, конфиг можно подменить через `CONFIG_PATH`.

## Разработка

```bash
go build ./...                                        # сборка
go vet ./...                                          # статический анализ
go test -count=1 -p 1 ./internal/... ./tests/...       # все тесты
go test -count=1 -p 1 ./internal/... ./tests/... -cover # с покрытием
```

Быстрые проверки по частям:

```bash
go test ./internal/cache/... ./internal/config/... ./internal/i18n/...
go test ./internal/parser/... ./tests/...
go test ./internal/telegram/...
```

`-p 1` не обязателен семантически, но заметно снижает пиковое потребление памяти на слабых машинах и в CI.

### Паритет с TypeScript-ботом

Старый TS-бот живёт в ветке `go` этого же репозитория. `scripts/paritycheck` читает её прямо из git и сравнивает три поверхности: имена Telegram-команд, корни callback-данных и подписи всех кнопок.

```bash
go run ./scripts/paritycheck                 # сравнить с веткой go
go run ./scripts/paritycheck -ts-ref origin/go
go run ./scripts/paritycheck -update         # перегенерировать TS-фикстур
go run ./scripts/paritycheck -dump-go        # напечатать текущую поверхность Go
```

- `internal/telegram/testdata/parity/ts_surface.json` — зафиксированная поверхность TS;
- `internal/telegram/testdata/parity/known_differences.json` — список принятых отличий, **каждая запись обязана иметь `reason`**, иначе проверка падает;
- `internal/telegram/parity_test.go` перепроверяет поверхность офлайн, поэтому обычный `go test` ловит дрейф и без ветки TS.

Раскладки клавиатур хранятся golden-файлом `internal/telegram/testdata/keyboard_layouts.golden`: тест прогоняет все билдеры по матрице режимов и профилей отображения и падает при любом сдвиге кнопки.

```bash
go test ./internal/telegram -run Golden          # проверить раскладки
go test ./internal/telegram -update              # перегенерировать golden-файл
```

Те же проверки выполняет CI (`.github/workflows/go-ci.yml`): job `build` — сборка, vet, тесты; job `parity` — сравнение с веткой `go` и golden-раскладки.

## Структура проекта

```
cmd/bot/                 — entrypoint бота
cmd/migrate-pg/          — миграция данных из PostgreSQL в SQLite
internal/
  api/                   — gin REST API
  archive/               — SQLite-архив расписания
  cache/                 — файловый кэш расписания и события для уведомлений
  calendar/              — экспорт ICS
  config/                — загрузка YAML-конфигурации
  formatter/             — форматы вывода расписания (default, compact, visual, litolax)
  google/                — Google OAuth, service account, синхронизация дней
  i18n/                  — go-i18n, locales/ru.json
  image/                 — рендеринг расписания в PNG
  logger/                — zerolog + lumberjack
  model/                 — Group, Teacher, Day, Lesson, CallsSchedule
  notification/          — планировщик и события уведомлений
  parity/                — компаратор поверхностей TS ↔ Go
  parser/                — парсер v1 (таблицы) и парсер звонков
  parser/v2/             — парсер v2 (grid, валидация, diff)
  telegram/              — telego: команды, колбэки, меню, клавиатуры, сцены
  utils/                 — учебные недели, предметы
configs/config.example.yaml — шаблон конфигурации
docs/google-calendar.md     — инструкция по Google Calendar
migrations/                 — SQL-миграции SQLite
scripts/paritycheck/        — чекер паритета с TS-ботом
tests/                      — интеграционные тесты парсера, кэша и архива
cache/rasp/                 — JSON-кэш расписания (создаётся в рантайме)
```

## Боты

| Платформа | Ссылка |
|-----------|--------|
| Telegram | https://t.me/mgke_slave_bot |

## Лицензия

MIT. При создании своей версии обязательно указывайте авторство оригинального проекта.
