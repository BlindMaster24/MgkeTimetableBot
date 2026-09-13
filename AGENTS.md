# Repository Guidelines

## Project Structure & Module Organization
- Entrypoint: `cmd/bot/main.go` — wires config, logger, i18n, cache, archive, API, Telegram bot, notification scheduler, parser loop.
- `cmd/migrate-pg/` — one-off migration of chats and subscriptions from PostgreSQL to SQLite.
- Internal packages (`internal/`):
  - `config/` — YAML configuration plus reflection-based `MGKE_*` environment overrides.
  - `logger/` — zerolog with lumberjack rotation.
  - `i18n/` — go-i18n with embedded `locales/*.json` (Russian).
  - `model/` — domain types: Group, Teacher, Day, Lesson, CallsSchedule.
  - `parser/` — goquery HTML parser (groups, teachers, bell schedule, calls).
  - `archive/` — SQLite repository for timetable archive (modernc.org/sqlite, pure Go).
  - `cache/` — file-backed RaspCache with in-memory state, hit/miss metrics and the event bus drained by the notifier.
  - `telegram/` — telego bot: commands, callbacks, menus, keyboards, scenes, Google Calendar menus.
  - `api/` — gin REST API: info, groups, teachers, group/teacher by name, parser-health, health, Google OAuth callback.
  - `health/` — in-memory metrics tracker (parser, calendar sync, API) with threshold-based alert evaluation.
  - `google/` — Google Calendar OAuth2 user client, Service Account sync, lesson event builders.
  - `image/` — fogleman/gg timetable PNG renderer.
  - `calendar/` — ICS calendar export (hand-rolled, no external lib).
  - `notification/` — cron scheduler (robfig/cron), event-to-message mapping and health alert dispatch.
  - `formatter/` — output formats: default, compact, visual, litolax.
  - `parity/` — TS ↔ Go surface comparator used by `scripts/paritycheck` and the offline parity test.
  - `utils/` — academic week index, subject list.
- `docs/` — user-facing instructions (Google Calendar). `scripts/paritycheck/` — parity checker binary.
- `README.md` / `README.en.md` — mirrored documentation, kept in sync by `tests/docs_test.go`.
- `Dockerfile`, `docker-compose.yml`, `.dockerignore` — container build; runtime config comes from env vars, state lives in the `/data` volume.

## Architecture Overview (Flow)
- External inputs arrive via Telegram bot (long polling) or HTTP API.
- Bot commands are routed in `internal/telegram/bot.go` via command name or text matching; messages landing in an input scene go through the scene registry in `internal/telegram/scenes.go`.
- Parser fetches HTML from the college site, normalizes via goquery, emits events into `cache/`.
- `cmd/bot/main.go` drains those events after every parse into `internal/notification`, then syncs only the changed days into Google Calendar and finally reconciles calendars that fell behind.
- `internal/health` records parser, calendar and API outcomes; the scheduler polls it and messages admins while an alert is active.
- Parsed data is stored in `cache/` (file-backed JSON in `cache/rasp/`) and `archive/` (SQLite).
- Output is delivered through telego (Telegram) or gin (HTTP API).

## Where to Add New Code
- New bot command: add struct in `internal/telegram/commands.go` (or `admin_commands.go` / `extras.go` for admin and service commands), implement `Command` interface, register in `registerAll()`.
- New callback: add struct in `internal/telegram/callbacks.go`, implement `Callback` interface, register in `registerAll()`.
- New menu: add one spec to the table in `internal/telegram/menus.go`; do not hand-register its command or text handlers.
- New input scene: add a typed scene constant and a route in `internal/telegram/scenes.go`; never compare raw scene strings in handlers.
- New API endpoint: add handler in `internal/api/server.go`, register route in `routes()`; both README files must document it (`tests/docs_test.go`).
- New metric or alert: record it in `internal/health` and map the alert key to a message in `internal/notification/health.go`.
- New config field: add it with a `yaml` tag — the `MGKE_*` environment name is derived from that tag automatically; document it in `configs/config.example.yaml` and in both README files.
- New parser type: add file in `internal/parser/`.
- New locale string: add key to `internal/i18n/locales/ru.json`, use `b.loc("key")` in code.

## Build, Test, and Development Commands
- Runtime: Go 1.27.1+ — the minimum is pinned in `go.mod` (`go 1.27.1`); CI reads it through `go-version-file: go.mod`.
- Package manager: Go modules (go.mod).
- `go mod tidy` — sync dependencies (it drops a redundant `toolchain` line; keep go.mod tidy).
- `go build -o bot ./cmd/bot/` — build binary.
- `go run ./cmd/bot/ -config configs/config.yaml` — run bot.
- `go test -count=1 -p 1 ./internal/... ./tests/...` — run all tests (`-p 1` keeps peak memory down).
- `go test -count=1 ./internal/... ./tests/... -cover` — run with coverage.
- `go vet ./...` — static analysis.
- `go clean -cache` — clean build cache.

## Verification Checklist
- `go vet ./...` for static analysis after any changes.
- `go test -count=1 -p 1 ./internal/... ./tests/...` before commit.
- `go build -o bot ./cmd/bot/` to verify binary compiles.
- `go run ./scripts/paritycheck` to verify the Telegram surface still matches the old TypeScript bot.
- `go test -count=1 ./tests/` to verify the documentation still matches the bot surface, API routes and config keys.
- `go run ./cmd/bot/ -config configs/config.yaml` for a smoke run (manual).

## Parity with the old TypeScript bot
- The old bot lives on the `go` branch; `scripts/paritycheck` reads it straight from git (`-ts-ref`, default `go`).
- It compares three surfaces: Telegram command names, callback roots, and every keyboard button label. `-dump-go` prints the live Go surface, `-allowlist` overrides the known-differences file.
- `internal/telegram/testdata/parity/ts_surface.json` is the TypeScript fixture; `go run ./scripts/paritycheck -update` regenerates it.
- Every accepted difference must be listed in `internal/telegram/testdata/parity/known_differences.json` with a reason; an entry without a reason is an error.
- `internal/telegram/parity_test.go` re-checks the same surface offline, so plain `go test` catches drift.
- Keyboard layouts are golden files: `go test ./internal/telegram -run Golden -update` rewrites `internal/telegram/testdata/keyboard_layouts.golden`.

## Menus
- Menus are declared in `internal/telegram/menus.go`: one entry per menu holds its scene, prompt, opening button texts, keyboard builder and text items.
- `registerMenus` turns a spec into its `/command` (when it has a public name) or a text-only opener, plus all item handlers.
- Adding a menu means adding one spec; menus without a scene or with items outside their scene fail `menus_test.go`.

## Current Features Snapshot
- Telegram bot via telego long polling.
- Commands: 63 total — schedule and setup (`/start`, `/help`, `/setup`, `/day`, `/week`, `/calls`, `/group`, `/teacher`, `/image`, `/cabinet`, `/history`, `/archive`, `/alias`, `/comparegroups`, `/groups`, `/teachers`, `/ics`, `/google_calendar`, …), admin (`/debug`, `/send`, `/trigger`, `/forceparse`, `/noticedebug`, `/sql`, `/restart`, …) and menu text pseudo-commands. The full list is in `README.md` and enforced by `scripts/paritycheck`.
- Keyboard buttons match command text via i18n keys.
- Parser (table-based) for groups, teachers and the bell schedule, with change detection.
- File-backed cache with JSON persistence in `cache/rasp/`.
- SQLite archive for historical schedule data.
- REST API (gin) with /api/info, /api/groups, /api/teachers, /api/group/:name, /api/teacher/:name, /api/parser-health, /api/health, plus the Google OAuth callback route. `/api/health` returns 503 while alerts are active.
- Health metrics and Telegram alerts for parser lag, calendar sync failures and API 5xx bursts, tuned by the `health` config section.
- ICS calendar export for schedule events.
- Google Calendar sync via Service Account: per-chat accounts (`google_accounts`) and calendars (`google_calendars`) in `bot_chats.db`, event-driven day sync plus a reconcile pass for days a calendar missed; every day is cleared before new events are written and lesson times come from the bell schedule.
- Notification scheduler (robfig/cron): end-of-lesson checks, new-week and changed-day notices, bell-schedule changes, parser errors; per-chat toggles in the notice menu.
- Image generation via fogleman/gg (pure Go, no CGO).
- Most user-facing strings live in `internal/i18n/locales/ru.json`; some menu texts are inline literals kept verbatim for TS parity. New strings go to `ru.json`, never as new inline literals.

## Environment Configuration
- `config.LoadWithEnv` reads the YAML file and then applies `MGKE_*` environment overrides (`config.ApplyEnv`).
- Names are derived from `yaml` tags: section path joined with `_`, uppercased, prefixed with `MGKE_`. A handful of fields also keep legacy aliases from their `env` tags (`TG_TOKEN`, `DB_PATH`, `HTTP_PORT`, `LOG_LEVEL`, `ENCRYPT_KEY`); the `MGKE_*` name wins when both are set.
- Precedence: `-config` flag, then environment variables, then the file. `CONFIG_PATH` only selects the file.
- Scalars, `[]string` / `[]int64` and fixed-size scalar arrays (`[2]int`) are supported; an env override for a complex field (maps, slices of structs, nested tables like `timetable.weekdays`) is a startup error, keep those in YAML.
- `config.EnvNames()` lists every supported variable; `tests/docs_test.go` fails if a config key disappears from the READMEs.

## Health Metrics and Alerts
- `internal/health.Tracker` holds counters (parser, calendar sync, API) and evaluates `health.Alert` values against `health.Thresholds`; `Thresholds.WithDefaults()` fills zero values.
- `cmd/bot/main.go` records parser successes/failures and calendar sync results; the gin middleware in `internal/api/server.go` records requests and 5xx responses.
- `GET /api/health` returns the snapshot (200 healthy, 503 while alerting) — the Docker healthcheck relies on that code.
- `notification.HealthNotifier` runs on a cron entry, messages admins once per alert per `health.cooldown_minutes` and sends a recovery message when the alert clears.
- Thresholds live in the `health` config section (`disabled`, `check_minutes`, `parser_*`, `calendar_*`, `api_*`); add new alert keys to both `internal/health` and `internal/notification/health.go`.
- `internal/parser` reports diagnostics for every source through `parser.Report`: each required probe names the CSS selector that must match, and a parse that finds nothing, shrinks suspiciously or falls back keeps the previous cache entry instead of overwriting it. The sink is wired in `cmd/bot/main.go` into `health.Tracker.ParserReport` (alert `parser_layout`, `health.parser_layout_failures`) and into the bot, which renders it in `/parserLogs`.

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
- Metrics check: `go test ./internal/health/... ./internal/api/...`
- Docs guard: `go test ./tests/`
- Parser check: `go test ./internal/parser/... ./tests/...`
- Bot check: `go test ./internal/telegram/...`
- Full suite: `go test -count=1 -p 1 ./internal/... ./tests/...`
- Parity surface: `go run ./scripts/paritycheck`; keyboard layouts: `go test ./internal/telegram -run Golden`
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
- `configs/config.yaml` is gitignored; start from the tracked template: `cp configs/config.example.yaml configs/config.yaml` and fill in real values (tokens, keys).
- Keep secrets out of git and avoid committing local DB files (e.g., `sqlite3.db`) or `.env` files.
- Prefer environment variables over editing YAML for deployment-specific values (tokens, admin IDs, ports, storage paths).
- Storage paths are configurable: `db_path` (archive), `chat_db_path` (chats and Google data), `cache_dir` (file cache).
- Run the bot with an explicit config path: `go run ./cmd/bot/ -config configs/config.yaml` (`-config` wins over `CONFIG_PATH`, both default to `configs/config.yaml`).

## Bot Behavior Notes
- Core flow: `cmd/bot/main.go` starts config, logger, cache, archive, parser goroutine, API server, then Telegram bot.
- Schedule parsing relies on site HTML; keep selectors tolerant to layout changes.
- Parser cache lives under `./cache/rasp/` (override with `cache_dir`) as JSON files; the chat database is `./bot_chats.db` (override with `chat_db_path`).
- Telegram bot uses telego long polling with command routing by name and text matching.
- All bot text goes through i18n: `b.loc("key")` returns localized string.
