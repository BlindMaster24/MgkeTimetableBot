# Правила репозитория

[English version](AGENTS.md)

Это русская версия правил для разработчиков и агентов. Она повторяет структуру
`AGENTS.md` и фиксирует обязательные правила; полные инварианты, перечисления
селекторов, метрик и алертов остаются в английском файле, который является
источником истины при расхождении. Любая правка поведения обновляет **оба**
файла в одном коммите.

## Структура проекта

- Точка входа — `cmd/bot/main.go`: конфиг, логгер, i18n, кэш, архив, API, бот,
  планировщик уведомлений, цикл парсера. Вся сборка — в `internal/app`.
- `cmd/migrate-pg/` — разовая миграция чатов и подписок из PostgreSQL в SQLite.
- Пакеты `internal/`:
  - `config/` — YAML и переопределения `MGKE_*` через рефлексию;
  - `logger/` — zerolog + ротация lumberjack;
  - `i18n/` — go-i18n, `locales/ru.json` — единственный дом русских текстов;
  - `model/` — доменные типы (Group, Teacher, Day, Lesson, CallsSchedule);
  - `parser/` — разбор HTML (группы, преподаватели, звонки, диагностика);
  - `archive/` — SQLite-архив (modernc.org/sqlite, чистый Go), `sync.go`
    пишет дни кэша и переписывает изменившиеся, `retention.go` — порог и чистка;
  - `cache/` — файловый кэш, метрики попаданий и шина событий;
  - `telegram/` — telego: команды, колбэки, меню, клавиатуры, сцены;
  - `api/` — gin: info, groups, teachers, group/teacher by name, parser-health,
    health, OAuth-callback; `/metrics` живёт отдельно и не под ключом `/api`;
  - `apiprobe/` — живая проверка собственных ручек для алерта `api_errors`;
  - `health/` — метрики и алерты, персист в `bot_state`, история инцидентов;
  - `google/` — OAuth2 пользователя, service account, события;
  - `image/` — PNG на fogleman/gg, голдены и `CheckGoldenLayout`;
  - `calendar/` — ICS без внешних библиотек;
  - `notification/` — cron, события, алерты, напоминания; отправки пейсятся;
  - `formatter/` — четыре формата вывода, тексты из локалей;
  - `parity/` — сравнение поверхности с TS-ботом;
  - `preflight/` — проверки перед деплоем (конфиг, ключи, webhook, локаль,
    хранилище, метрики, сайт, картинка);
  - `tgstub/` — заглушка Bot API только на loopback;
  - `testgolden/` — нормализация golden-текстов (только тесты);
  - `timetable/` — типизированные дни и уроки, даты `дд.мм.гггг`;
  - `reminder/`, `rooms/`, `schedulediff/`, `search/`, `utils/` — чистые функции.
- `docs/` — пользовательские инструкции, каждая пара RU/EN.
- `README.md` и `README.en.md` — зеркала, синхронность проверяет `tests/`.
- `Dockerfile`, три compose-файла и `deploy/` — сборка, long polling, webhook
  за Caddy и nginx, набор мониторинга.

## Поток работы

1. Внешние входы — Telegram (long polling или webhook) и HTTP API.
2. Команды маршрутизируются в `internal/telegram/bot.go`; сцены — `scenes.go`.
3. `Bot.Run` выбирает транспорт; контракт webhook описан в `AGENTS.md`.
4. Парсер забирает HTML, нормализует goquery и кладёт события в кэш.
5. `cmd/bot/main.go` после каждого разбора отдаёт события в `notification`,
   затем синхронизирует изменённые дни в Google и добирает отставшие календари.
6. `internal/health` пишет результаты парсера, календаря и API; планировщик
   сообщает админам, пока алерт активен.
7. Данные лежат в `cache/rasp/*.json` и в SQLite-архиве.

Дни недели на сайте публикуются блоками: парсер читает **все** блоки и мержит
дни по дате. Каждый день в кэше обязан иметь дату `дд.мм.гггг`; колонка без
разбираемой даты пропускается и считается в диагностике.

## Куда добавлять код

- Новая команда: структура в `commands.go` (пользовательская) или в файле
  фичи; реализовать `Command`, зарегистрировать в `registerAll()`. Имя должно
  подходить Telegram (`a-z`, `0-9`, `_`, 1–32 символа), описание 3–256;
  `validateBotCommands` отбивает остальное до `setMyCommands`.
- Новый колбэк: структура в `callbacks.go`, реализовать `Callback`.
- Новое меню: одна запись в таблице `menus.go`.
- Новая сцена ввода: константа и маршрут в `scenes.go`.
- Новая ручка API: обработчик в `internal/api/server.go` и маршрут в `routes()`,
  оба README и `tests/docs_test.go`.
- Новая метрика: коллектор в `api/metrics.go`, имя в `metricFamilies`,
  состояние — через `MetricsSources`, значения только числа.
- Новый ключ конфига: `yaml`-тег (имя `MGKE_*` выводится из него), шаблон и оба
  README.
- Новый источник парсинга: файл в `internal/parser/` с `Report`: обязательные
  пробы называют селектор, пропуски считаются через `probeSkipped`.
- Новый текст: ключ в `ru.json`, в коде только `b.loc`/`locData`. Данные остаются в Go: кириллическая буква в `mathAllowedCharacters`, входные регулярки в `inputcmds.go`, фикстуры поверхности в `surface.go` и эталонные примеры в `image/golden.go` — это то, что код сопоставляет или рисует, а не подписи.
- Новый алерт: `internal/health` + текст в `internal/notification/health.go`.
- Новый сценарий e2e: `tests/e2e_test.go` или `internal/telegram/*_e2e_test.go`.

## Сборка, тесты, проверки

- Go 1.27.4+; версия живёт только в `go.mod`.
- `go build -o bot ./cmd/bot/`, `go run ./cmd/bot/ -config configs/config.yaml`.
- `go test -count=1 -p 1 ./internal/... ./tests/...` — весь набор.
- `go vet ./...`, `gofmt -l .` (любой вывод — провал).
- `go run ./scripts/racecheck` — тот же набор под `-race` (нужны cgo и компилятор).
- `go run ./scripts/tgstub -listen 127.0.0.1:18090` — заглушка Bot API.
- `go run ./scripts/devbot` — офлайн-профиль: заглушка плюс бот, состояние в `./.dev/`.
- `go run ./scripts/paritycheck` — сверка поверхности с веткой `old`.
- `go run ./scripts/preflight -config configs/config.yaml` — проверка перед деплоем.
- `go run ./scripts/monitoringcheck -strict` — правила, дашборд, копии для Kubernetes.
- `go test ./internal/image -update` — принять новую вёрстку PNG (единственный путь).
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — уязвимости зависимостей.

## CI и релизы

- `ci.yml`: `quality`, `test`, `newest-toolchain`, `race`, `fuzz`, `parity`,
  `monitoring`, `preflight`, `container`, `devbot`, `workflow-lint`.
- Платформы — Ubuntu, Windows, macOS; `GOTOOLCHAIN: local`; экшены закреплены SHA.
- `release.yml` сначала проверяет ревизию, затем публикует образ в GHCR и
  бинарники для linux/amd64, linux/arm64, windows/amd64, darwin/arm64.
- Версия, коммит и дата приходят через `-X main.*` и видны в `/api/health`.
- `security.yml` — `govulncheck` по расписанию, вне блокировки PR.

## Чек-лист перед коммитом

- `go vet ./...`; `go test -count=1 -p 1 ./internal/... ./tests/...`;
  `go build -o bot ./cmd/bot/`.
- `go run ./scripts/paritycheck` при изменении поверхности.
- `go test ./internal/image/` после правок рендера; `-update` — только осознанно.
- `go test ./internal/app/ -run EndToEnd` после правок диагностики парсера.
- `go test ./tests/` — документация совпадает с командами, ручками и ключами.
- Фаззинг затронутых пакетов: `-fuzztime 30s` для `parser`, `schedulediff`,
  `search`, `reminder`, `rooms`, webhook.
- Не присваивать `time.Local`; часы инжектируются (`Bot.SetNow`,
  `EventNotifier.SetNow`), иначе `tests/timezone_test.go` падает.
- Новый файл теста — точная команда `go test ./path/...` в описании PR.

## Паритет с TypeScript-ботом

- Старый бот — ветка `old`, `scripts/paritycheck` читает её из git.
- Сравниваются команды, корни колбэков, подписи кнопок и тексты.
- `internal/telegram/testdata/parity/ts_surface.json` — фикстура TS;
  `known_differences.json` — принятые отличия, **каждое с причиной**.
- Раскладки клавиатур и тексты сообщений — golden-файлы; даты и недели
  нормализуются `testgolden`.
- PNG — golden-изображения; сравнение попиксельное с допуском 8.

## Меню

- Меню объявляются в `menus.go`: одна запись — сцена, подсказка, тексты
  открытия, клавиатура и пункты.
- `registerMenus` превращает запись в команду или текстовый вход и обработчики.

## Возможности (снимок)

- Telegram через telego: long polling по умолчанию, webhook по конфигу.
- 69 команд: расписание, настройки, админские и текстовые псевдокоманды.
- Парсер таблиц групп, преподавателей и звонков с диагностикой и защитой кэша.
- Файловый кэш JSON и SQLite-архив истории.
- REST API (gin) с ключами, лимитами и `/api/health` (503 при алертах).
- `/metrics` для Prometheus: только агрегаты, без секретов и ID.
- Мониторинг: 14 алертов с promtool-тестами, дашборд, Kubernetes, Alloy.
- Алерты в Telegram: парсер, календарь, API, бэкапы; кнопки починки.
- ICS-экспорт и Google Calendar (service account, права reader/writer).
- Планировщик уведомлений: конец пары, новая неделя, изменения, звонки.
- PNG-рендер расписания, голдены, preflight-проверка вёрстки.
- Русские тексты — в `ru.json`; новые строки только там.

## Конфигурация и безопасность

- `configs/config.yaml` в gitignore; шаблон — `configs/config.example.yaml`.
- Секреты — переменными окружения; `encrypt_key` не короче 32 символов.
- `db_path` и `chat_db_path` обязаны указывать на разные файлы.
- `configs/config.yaml` запускается только явным путём (`-config` важнее
  `CONFIG_PATH`).

## Поведение бота

- Порядок старта: конфиг, логгер, кэш, архив, парсер, API, бот.
- Парсер терпим к вёрстке: имя — из ближайшего заголовка, дни — из `th` с
  учётом `colspan`, аудитория — из текста ячейки как фолбэк.
- Кэш — `./cache/rasp/`, база чатов — `./bot_chats.db` (оба настраиваются).
- Весь текст — через i18n: `b.loc("key")`.

## Документация — часть изменения

- Пользовательское поведение — оба README в том же коммите.
- Поведение для разработчика — этот файл и `AGENTS.md`.
- Новый ключ конфига — шаблон и оба README.
- Новый алерт, метрика или панель — `deploy/monitoring/` и preflight.
- Новый текст — голден-сценарий и `ru.json`.
- Новый парсер-диагностика, команда, ручка, переменная или джоба CI —
  документируется там, где живёт.

## Коммиты и PR

- Короткие императивные сообщения на английском, до 72 символов,
  Conventional Commits: `type` обязателен, `scope` — короткий, описание без точки.
- `git status` перед коммитом, файлы стейджить явно, без `git add .`.
- В PR — описание и команды, которые вы запускали.
