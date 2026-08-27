# МГКЦТ Бот расписаний

[![CI](https://github.com/BlindMaster24/MgkeTimetableBot/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/BlindMaster24/MgkeTimetableBot/actions/workflows/ci.yml)

## Описание
МГКЦТ Бот расписаний — бот для удобного просмотра расписания в Минском государственном колледже цифровых технологий.

Написан на **Go** с использованием:
- [telego](https://github.com/mymmrac/telego) — Telegram Bot API
- [gin](https://github.com/gin-gonic/gin) — HTTP сервер
- [goquery](https://github.com/PuerkitoBio/goquery) — парсинг HTML
- [zerolog](https://github.com/rs/zerolog) — логирование
- [lumberjack](https://github.com/natefinch/lumberjack) — ротация логов
- [go-i18n](https://github.com/nicksnyder/go-i18n) — интернационализация
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — SQLite (чистый Go, без CGO)
- [fogleman/gg](https://github.com/fogleman/gg) — генерация изображений
- [golang-ical](https://github.com/arran4/golang-ical) — ICS экспорт

## Требования
- Go 1.22+
- SQLite (для archive)

## Установка и запуск

1. Клонируйте репозиторий:
    ```bash
    git clone https://github.com/BlindMaster24/MgkeTimetableBot.git
    cd MgkeTimetableBot
    git checkout main
    ```

2. Скопируйте и заполните конфиг:
    ```bash
    cp configs/config.yaml configs/config.local.yaml
    # Отредактируйте configs/config.local.yaml — добавьте токен Telegram, ключи и т.д.
    ```

3. Установите зависимости:
    ```bash
    go mod tidy
    ```

4. Соберите бинарник:
    ```bash
    go build -o bot ./cmd/bot/
    ```

5. Запустите:
    ```bash
    ./bot -config configs/config.local.yaml
    ```

    Или через `go run`:
    ```bash
    go run ./cmd/bot/ -config configs/config.yaml
    ```

## Тесты

```bash
go test ./internal/... ./tests/... -v        # все тесты
go test ./internal/... ./tests/... -cover    # с покрытием
go vet ./...                                 # статический анализ
```

## Структура проекта

```
cmd/bot/main.go              — entrypoint
internal/
  config/                    — YAML конфигурация
  logger/                    — zerolog + lumberjack
  i18n/                      — go-i18n + locales/ru.json
  model/                     — Group, Teacher, Day, Lesson
  parser/                    — goquery v1 парсер
  parser/v2/                 — goquery v2: grid, validate, diff
  archive/                   — SQLite репозиторий
  cache/                     — file-backed RaspCache
  telegram/                  — telego бот
  api/                       — gin REST API
  google/                    — Google Calendar
  image/                     — PNG рендеринг
  calendar/                  — ICS экспорт
configs/config.yaml          — конфигурация
migrations/001_init.sql      — SQLite миграция
```

## Конфигурация

Основные настройки в `configs/config.yaml`:

| Поле | Описание |
|------|----------|
| `telegram.token` | Токен Telegram бота |
| `telegram.admin_ids` | ID администраторов |
| `db_path` | Путь к SQLite базе |
| `http.port` | Порт HTTP API |
| `parser.enabled` | Включить парсер |
| `parser.v2.enabled` | Включить v2 парсер |
| `calendar.ics.enabled` | Включить ICS экспорт |

## Google Calendar

Инструкция по настройке: [docs/google-calendar.md](docs/google-calendar.md).

## Боты

| Платформа | Ссылка |
|-----------|--------|
| Telegram | https://t.me/mgke_slave_bot |

## Лицензия
MIT. При создании своей версии обязательно указывайте авторство оригинального проекта.
