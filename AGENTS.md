# Repository Guidelines

## Project Structure & Module Organization
- Entrypoint: `cmd/bot/main.go` — wires config, logger, i18n, cache, archive, API, Telegram bot, parser loop.
- Internal packages (`internal/`):
  - `config/` — YAML + env configuration via `gopkg.in/yaml.v3`.
  - `logger/` — zerolog with lumberjack rotation.
  - `i18n/` — go-i18n with embedded `locales/*.json` (Russian).
  - `model/` — domain types: Group, Teacher, Day, Lesson, CallsSchedule.
  - `parser/` — goquery v1 HTML parser (groups, teachers).
  - `parser/v2/` — goquery v2 parser with grid detection, validation, diff.
  - `archive/` — SQLite repository for timetable archive (modernc.org/sqlite, pure Go).
  - `cache/` — file-backed RaspCache with in-memory state, hit/miss metrics.
  - `telegram/` — telego bot: commands, callbacks, keyboards.
  - `api/` — gin REST API: groups, teachers, parser-health.
  - `google/` — Google Calendar OAuth2 + Service Account.
  - `image/` — fogleman/gg timetable PNG renderer.
  - `calendar/` — ICS calendar export.

## Architecture Overview (Flow)
- External inputs arrive via Telegram bot (long polling) or HTTP API.
- Bot commands are routed in `internal/telegram/bot.go` via command name or text matching.
- Parser fetches HTML from the college site, normalizes via goquery, emits events.
- Parsed data is stored in `cache/` (file-backed JSON in `cache/rasp/`) and `archive/` (SQLite).
- Output is delivered through telego (Telegram) or gin (HTTP API).

## Where to Add New Code
- New bot command: add struct in `internal/telegram/commands.go`, implement `Command` interface, register in `registerAll()`.
- New callback: add struct in `internal/telegram/callbacks.go`, implement `Callback` interface, register in `registerAll()`.
- New API endpoint: add handler in `internal/api/server.go`, register route in `routes()`.
- New parser type: add file in `internal/parser/` or `internal/parser/v2/`.
- New locale string: add key to `internal/i18n/locales/ru.json`, use `b.loc("key")` in code.

## Build, Test, and Development Commands
- Runtime: Go 1.22+ (LTS).
- Package manager: Go modules (go.mod).
- `go mod tidy` — sync dependencies.
- `go build ./cmd/bot/` — build binary.
- `go run ./cmd/bot/ -config configs/config.yaml` — run bot.
- `go test ./internal/... ./tests/... -v` — run all tests.
- `go test ./internal/... ./tests/... -cover` — run with coverage.
- `go vet ./...` — static analysis.
- `go clean -cache` — clean build cache.

## Verification Checklist
- `go vet ./...` for static analysis after any changes.
- `go test ./internal/... ./tests/... -cover` before commit.
- `go build ./cmd/bot/` to verify binary compiles.
- `go run ./cmd/bot/ -config configs/config.yaml` for a smoke run (manual).

## Current Features Snapshot
- Telegram bot via telego long polling.
- Commands: /start, /help, /cancel, /setup, /day, /week, /calls, /about, /group, /teacher, /settings, /image.
- Keyboard buttons match command text via i18n keys.
- Parser v1 (table-based) and v2 (grid-based) with validation and diff.
- File-backed cache with JSON persistence in `cache/rasp/`.
- SQLite archive for historical schedule data.
- REST API (gin) with /api/groups, /api/teachers, /api/parser-health.
- ICS calendar export for schedule events.
- Google Calendar sync via Service Account.
- Image generation via fogleman/gg (pure Go, no CGO).
- All user-facing strings in `internal/i18n/locales/ru.json` — zero hardcoded Russian in Go code.

## Parser v2
- Enable in `configs/config.yaml` via `parser.v2.enabled`.
- Keep `parser.v2.fallback_to_v1` true for safe rollout.
- Use `parser.v2.week_policy: preferCurrent` to avoid switching to a future week.
- Use `parser.v2.sunday_hold_current: true` to avoid switching on Sundays.
- Use `parser.v2.hash_mode: tables` to reduce noise from layout changes.
- Use `parser.v2.quarantine` to block suspicious updates (too few lessons).
- Metrics options: `parser.v2.metrics.enabled`, `parser.v2.metrics.dir`.

## Coding Style & Naming Conventions
- Go is strict; keep `go vet` clean.
- Indentation: tabs (Go standard).
- Naming: `camelCase` vars/functions, `PascalCase` exported types/methods, `SCREAMING_SNAKE_CASE` constants.
- Do not add code comments.
- Keep package boundaries: cross-package access goes through exported APIs.
- Russian text only in `internal/i18n/locales/ru.json` — never in Go source.

## Testing Guidelines
- Test runner: `go test`.
- Name tests `*_test.go` co-located with source files.
- Keep tests deterministic and offline — no network/API calls.
- Prefer pure-function tests for parser, cache, and utility functions.
- Use `httptest` for API handler tests.
- If you add a new test file, list the exact `go test ./path/...` command in the PR description.

## Test Execution Order
- Fast local check: `go test ./internal/cache/... ./internal/config/... ./internal/i18n/...`
- Parser check: `go test ./internal/parser/... ./tests/...`
- Bot check: `go test ./internal/telegram/...`
- Full suite: `go test ./internal/... ./tests/... -cover`
- Static analysis: `go vet ./...`

## Commit & Pull Request Guidelines
- Follow existing commit style: short, imperative summaries in English, <= 72 chars.
- Prefer a `type:` prefix (`feat`, `fix`, `docs`, `refactor`, `chore`) when it fits the change.
- Commit messages must conform to Conventional Commits.
- Core rules:
  - `type` is required and must be lowercase.
  - `scope` is optional and should be a short area (e.g., `parser`, `telegram`, `api`, `cache`).
  - `description` is required, imperative, no trailing period.
- PRs should include a clear description and the commands you ran.
- Always check `git status` before committing.
- Stage files explicitly; do not use `git add .`.

## Configuration & Security
- Copy `configs/config.yaml` and fill in real values (tokens, keys).
- Keep secrets out of git and avoid committing local DB files (e.g., `sqlite3.db`).
- Config example: `configs/config.yaml` with all production values from TS config.

## Bot Behavior Notes
- Core flow: `cmd/bot/main.go` starts config, logger, cache, archive, parser goroutine, API server, then Telegram bot.
- Schedule parsing relies on site HTML; keep selectors tolerant to layout changes.
- Parser cache lives under `./cache/rasp/` as JSON files.
- Telegram bot uses telego long polling with command routing by name and text matching.
- All bot text goes through i18n: `b.loc("key")` returns localized string.
