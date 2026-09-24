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
  - `archive/` — SQLite repository for timetable archive (modernc.org/sqlite, pure Go); `migrations/*.sql` is embedded and applied by `Repository.EnsureSchema` from `New`.
  - `cache/` — file-backed RaspCache with in-memory state, hit/miss metrics and the event bus drained by the notifier.
  - `telegram/` — telego bot. `bot.go` registers everything and declares the `archiveStore` interface the bot depends on; `chat.go` + `chat_queries.go` + `chat_subscriptions.go` hold the chat database; `commands.go`, `settings_text.go`, `callbacks.go`, `menus.go`, `weekcontrol.go` carry the user surface; `timetabledays.go` is the single owner of day selection (the `daySource` choice between cache and archive, the group/teacher target of a chat, `daysToMaps`) and `texthints.go` holds the small label helpers; one file per feature for the rest (`history.go`, `stats.go`, `archive.go`, `ics.go`, `calls_edit.go`, `calls_settings.go`, `calls_campus.go`, `compare_groups.go`, `test_subscriptions.go`, `debug_commands.go`, `google_calendar.go`, `google_store.go`, `admin.go`, `admin_commands.go`).
  - `api/` — gin REST API: info, groups, teachers, group/teacher by name, parser-health, health, Google OAuth callback.
  - `apiprobe/` — live probe of the bot's own API endpoints (targets built from the cache, statuses, response times, health summary) used by the `api_errors` alert button.
  - `health/` — metrics tracker (parser, calendar sync, API) with threshold-based alert evaluation; `state.go` persists it through the `StateStore` interface.
  - `google/` — Google Calendar OAuth2 user client, Service Account sync, lesson event builders.
  - `image/` — fogleman/gg timetable PNG renderer.
  - `calendar/` — ICS calendar export (hand-rolled, no external lib).
  - `notification/` — cron scheduler (robfig/cron), event-to-message mapping and health alert dispatch.
  - `formatter/` — output formats: default, compact, visual, litolax.
  - `parity/` — TS ↔ Go surface comparator used by `scripts/paritycheck` and the offline parity test.
  - `preflight/` — pre-deploy checks: config keys, credentials, locale keys, storage, the live site (dates, calls), a rendered PNG; `scripts/preflight` prints the report.
  - `testgolden/` — golden-text normalization (dates, week numbers, callback data) shared by the message golden tests; tests only.
  - `utils/` — academic week index, subject list.
- `docs/` — user-facing instructions (Google Calendar). `scripts/paritycheck/` — parity checker binary; `scripts/preflight/` — pre-deploy check; `scripts/racecheck/` — local race-detector runner that checks the cgo/C-compiler prerequisites first (`internal/racecheck` holds the plan logic).
- `README.md` / `README.en.md` — mirrored documentation, kept in sync by `tests/docs_test.go`.
- `Dockerfile`, `docker-compose.yml`, `.dockerignore` — container build; runtime config comes from env vars, state lives in the `/data` volume.
- `.github/workflows/ci.yml` — quality gates on Linux, Windows and macOS; `.github/workflows/release.yml` — releases and the GHCR image; `.github/workflows/security.yml` — the scheduled `govulncheck` scan; `.github/dependabot.yml` — weekly dependency bumps.

## Architecture Overview (Flow)
- External inputs arrive via Telegram bot (long polling) or HTTP API.
- Bot commands are routed in `internal/telegram/bot.go` via command name or text matching; messages landing in an input scene go through the scene registry in `internal/telegram/scenes.go`.
- Parser fetches HTML from the college site, normalizes via goquery, emits events into `cache/`.
- `cmd/bot/main.go` drains those events after every parse into `internal/notification`, then syncs only the changed days into Google Calendar and finally reconciles calendars that fell behind.
- `internal/health` records parser, calendar and API outcomes; the scheduler polls it and messages admins while an alert is active.
- Parsed data is stored in `cache/` (file-backed JSON in `cache/rasp/`) and `archive/` (SQLite).
- The site publishes several weeks at once, one content block per week: `internal/parser` walks every block (`findScopes`/`eachTable`), never the first one only, and merges the days of a group/teacher by date (`mergeDays`), so a repeated date is overwritten by the later block and the cache always holds every published week.
- Every cached day carries a plain `дд.мм.гггг` date in its `day` field: the formatters, the archive index, the notification bus and the Google/ICS exporters all parse it with `time.Parse("02.01.2006", ...)`. `cache.New` normalizes legacy labels ("Понедельник, 31.08.2026") on load, and the parser reports a required `th[colspan] with dd.MM.yyyy` probe so a page without parseable dates raises `parser_layout`.
- Telegram messages: a command or a text button always sends a new message; only an inline button edits the message it belongs to, using the message id of the callback query (`Update.MessageID`). Never route an edit through a message id stored in the chat row.
- Timetable days are fetched only through `internal/telegram/timetabledays.go`: callers pass an explicit `daySource` (`daysFromCache` for the day/week keyboard view, `daysFromArchive` for the navigation callbacks, `archiveDaysForRange` for `/archive` and `/history`, which must not fall back to the cache). The archive is reached through the `archiveStore` interface — never through a type assertion on `Bot.archive`, so `NewBot` accepts any implementation and `nil` means "no archive".
- Output is delivered through telego (Telegram) or gin (HTTP API).

## Where to Add New Code
- New bot command: add struct either to `internal/telegram/commands.go` (user surface) or to the feature file it belongs to, implement `Command` interface, register in `registerAll()`. Do not create a catch-all file: one feature, one file.
- New callback: add struct in `internal/telegram/callbacks.go`, implement `Callback` interface, register in `registerAll()`.
- New menu: add one spec to the table in `internal/telegram/menus.go`; do not hand-register its command or text handlers.
- New input scene: add a typed scene constant and a route in `internal/telegram/scenes.go`; never compare raw scene strings in handlers.
- New API endpoint: add handler in `internal/api/server.go`, register route in `routes()`; both README files must document it (`tests/docs_test.go`).
- New metric or alert: record it in `internal/health` and map the alert key to a message in `internal/notification/health.go`; a new API route worth probing goes into `apiprobe.Targets` in `internal/apiprobe/probe.go`.
- New config field: add it with a `yaml` tag — the `MGKE_*` environment name is derived from that tag automatically; document it in `configs/config.example.yaml` and in both README files.
- New parser source: add a file in `internal/parser/` that returns a `Report` from `report.go` — every required probe names the selector it expects.
- New end-to-end case: extend `tests/e2e_test.go` (real SQLite chat database) or `internal/telegram/*_e2e_test.go` (bot surface) — do not add a mock-only test where a real repository works.
- New pre-deploy check: add a `Check` to `internal/preflight` and cover it offline with an `httptest` fixture; the command must stay usable without network (`-skip-site`).
- New user-facing message: add it to the matching golden scenario (`internal/telegram/messages_golden_test.go` or `internal/notification/messages_golden_test.go`) in the same change.
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
- `gofmt -l .` — formatting check (CI fails on any output).
- `go run ./scripts/racecheck` — the race detector run CI performs; it verifies the cgo/C-compiler prerequisites first and prints the install steps (or `go run ./scripts/racecheck -check` to only report availability).
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` — lint the workflow files after touching `.github/workflows/`.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — dependency vulnerability scan (the scheduled `security.yml` job).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/bot/` — cross-compilation check; repeat for `linux/arm64` and `darwin/arm64` after touching platform-specific code.

## CI and Releases
- `.github/workflows/ci.yml` runs on every push to `main` and on every pull request. Independent jobs: `quality` (cross-platform build, vet and the tool binaries; gofmt and the release-target cross-compilation only on Linux), `test` (suite plus `coverage.out` artifact), `newest-toolchain` (the suite on the newest stable Go), `race` (`go run ./scripts/racecheck` — `-race` over the whole suite), `parity` (surface vs the `old` branch, golden keyboard layouts, docs guard), `container` (image build, container start, `/api/health` 200, build metadata, the container timezone and the graceful `SIGTERM` shutdown) and `workflow-lint` (`actionlint`).
- The platform-sensitive jobs (`quality`, `test`, `newest-toolchain`, `race`) carry a matrix over `ubuntu-latest`, `windows-latest` and `macos-latest` with `fail-fast: false`, and every workflow sets `defaults: run: shell: bash` so one command string works on all three. The platform-neutral gates (`parity`, `container`, `workflow-lint`) stay on Linux because they need Docker, a `git fetch` of the `old` branch or `jq`.
- Coverage is measured on all three operating systems but only the Linux run uploads the artifact and the summary, otherwise the artifact names would clash. The race job probes with `racecheck -check` first and, when Windows reports the detector as unavailable, warns instead of failing — `-race` builds through cgo and the Windows runner has no guaranteed C toolchain.
- `.github/workflows/security.yml` runs `govulncheck` on push to `main`, on a weekly schedule and on demand; it is deliberately outside CI so an advisory never blocks a pull request.
- Actions are pinned by commit SHA with the release tag in a comment; Dependabot bumps the SHA and the comment together, so keep the `# vN` comment when editing a `uses:` line.
- Every job sets `GOTOOLCHAIN: local`, so the pinned `go 1.27.1` from `go.mod` is the version the checks pass with and a newer toolchain is never substituted silently. The `newest-toolchain` job opts in explicitly with `go-version: stable`, so a Go release that breaks the suite shows up without moving the pinned minimum.
- `.github/workflows/release.yml` first runs `verify` (build, vet, tests on Linux, Windows and macOS) on the same revision and only then publishes: a push to `main` pushes the multi-arch image to `ghcr.io/blindmaster24/mgketimetablebot` as `:edge`/`:main`; a `v*` tag additionally builds linux/amd64, linux/arm64, windows/amd64 and darwin/arm64 archives, attaches them to a GitHub Release and gives the image `:1.2.3`, `:1.2` and `:latest`.
- The release build injects the tag, commit and build date through `-X main.version`, `-X main.commit` and `-X main.date`; `cmd/bot/main.go` logs them at startup, `internal/build` carries them into `GET /api/health`, `GET /api/info` and the admin `/debug` command. Bump the version by tagging, never by editing code.
- `internal/image` finds a font by probing the known Debian/Alpine, macOS and Windows paths and then scanning the system font directories, keeping only fonts that actually load; the runtime image installs `font-dejavu` so the PNG rendering works in the container too.
- The container runs in the college timezone (`TZ=Europe/Minsk`, `tzdata` installed, `TZ` passed by `docker-compose.yml`). Day boundaries, the academic week index and the notification crons are all derived from `time.Local`, so a container in UTC would fire notifications three hours late — `tests/container_test.go` guards both files.
- The Dockerfile builds its stage on `$BUILDPLATFORM` and cross-compiles with `GOOS=$TARGETOS GOARCH=$TARGETARCH`, so the multi-arch image never compiles Go under QEMU. Keep `CGO_ENABLED=0` — the bot is pure Go and the runtime image has no libc for cgo.
- Add a new CI job only when it fails for a reason a developer can reproduce locally, and mirror every new local check in the Verification Checklist above.

## Verification Checklist
- `go vet ./...` for static analysis after any changes.
- `go test -count=1 -p 1 ./internal/... ./tests/...` before commit.
- `go build -o bot ./cmd/bot/` to verify binary compiles.
- `go run ./scripts/paritycheck` to verify the Telegram surface still matches the old TypeScript bot.
- `go run ./scripts/preflight -config configs/config.yaml` before a deploy: it parses the live site and checks dates, calls, credentials, the locale keys, storage and PNG rendering in one run (exit code 1 on a failure, `-strict` also fails on warnings).
- `go test -count=1 ./tests/` to verify the documentation still matches the bot surface, API routes and config keys.
- `go build ./...`, `go vet ./...` and the suite on Windows and macOS as well (the CI matrix) — a change that only compiles on Linux reaches the platform-specific jobs red.
- `GOOS=<target> GOARCH=<arch> CGO_ENABLED=0 go build ./cmd/bot/` for the release targets after touching platform-specific code (`cmd/bot/signals_*.go` is the current example).
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` when dependencies change.
- The suite on the newest stable Go (the `newest-toolchain` job): install the latest Go release and run `go test -count=1 -p 1 ./internal/... ./tests/... ./cmd/...`.
- `go run ./cmd/bot/ -config configs/config.yaml` for a smoke run (manual); send `Ctrl+C`/`SIGTERM` and check the log ends with `shutdown complete`.

## Parity with the old TypeScript bot
- The old bot lives on the `old` branch; `scripts/paritycheck` reads it straight from git (`-ts-ref`, default `old`).
- It compares four surfaces: Telegram command names, callback roots, every keyboard button label, and user-visible message texts (every old text must exist in the Go sources or be documented). `-dump-go` prints the live Go surface, `-allowlist` overrides the known-differences file.
- `internal/telegram/testdata/parity/ts_surface.json` is the TypeScript fixture; `go run ./scripts/paritycheck -update` regenerates it.
- `internal/parser/testdata/old_parser/` holds gzipped snapshots of the live group and teacher pages next to the JSON the old TypeScript parsers produced for them; `old_parser_golden_test.go` parses the snapshots offline and fails on any difference, so the schedule parsers stay byte-identical to the old bot without a network or a Node toolchain.
- For side-by-side reading, export the branch once into a gitignored `old/` (`rm -rf old && mkdir old && git archive old | tar -x -C old`) and use the node helpers in `old/_tools` (`label-diff.js`, `string-diff.js`, `near-diff.js`, `command-diff.js`); `SURFACE_DUMP=old/_tools/go-commands.txt go test ./internal/telegram -run TestSurfaceDump` dumps the Go commands with their descriptions.
- Every accepted difference must be listed in `internal/telegram/testdata/parity/known_differences.json` with a reason; an entry without a reason is an error.
- `internal/telegram/parity_test.go` re-checks the same surface offline, so plain `go test` catches drift.
- Keyboard layouts are golden files: `go test ./internal/telegram -run Golden -update` rewrites `internal/telegram/testdata/keyboard_layouts.golden`.
- Message texts are golden files too: `internal/telegram/testdata/messages.golden` (day, week, calls) and `internal/notification/testdata/messages.golden` (notifications). `go test ./internal/telegram -update` and `go test ./internal/notification -update` rewrite them; `testgolden.Normalize` replaces dates, week numbers and callback data with tokens, and a guard test fails if a raw date reaches the output.

## Menus
- Menus are declared in `internal/telegram/menus.go`: one entry per menu holds its scene, prompt, opening button texts, keyboard builder and text items.
- `registerMenus` turns a spec into its `/command` (when it has a public name) or a text-only opener, plus all item handlers.
- Adding a menu means adding one spec; menus without a scene or with items outside their scene fail `menus_test.go`.

## Current Features Snapshot
- Telegram bot via telego long polling.
- Commands: 67 total — schedule and setup (`/start`, `/help`, `/setup`, `/day`, `/week`, `/calls`, `/group`, `/teacher`, `/image`, `/cabinet`, `/history`, `/archive`, `/alias`, `/comparegroups`, `/groups`, `/teachers`, `/ics`, `/google_calendar`, …), admin (`/debug`, `/send`, `/trigger`, `/forceparse`, `/noticedebug`, `/sql`, `/restart`, …) and menu text pseudo-commands. The full list is in `README.md` and enforced by `scripts/paritycheck`.
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
- `cmd/bot/main.go` records parser successes/failures and calendar sync results; the gin middleware in `internal/api/server.go` records every request, and for 5xx it keeps the method, the route, the response text (or the panic value) and a per-endpoint counter, so the `api_errors` alert names the endpoints that fail; the deliberate 503 of `GET /api/health` is marked `SelfProbe` and never counts as a failure, otherwise an active alert would keep itself alive.
- `GET /api/health` returns the snapshot (200 healthy, 503 while alerting) — the Docker healthcheck relies on that code.
- `notification.HealthNotifier` runs on a cron entry, messages admins once per alert per `health.cooldown_minutes` and sends a recovery message when the alert clears.
- `internal/health/state.go` persists counters and alert bookkeeping through the `health.StateStore` interface into the `bot_state` table of the chat DB: `Tracker.Restore`/`Tracker.Flush` in `cmd/bot/main.go`, the notifier restores and flushes its own alert state around `Check()`. A restart keeps `parser_stale` and the alert cooldown truthful.
- Thresholds live in the `health` config section (`disabled`, `check_minutes`, `parser_*`, `calendar_*`, `api_*`); add new alert keys to both `internal/health` and `internal/notification/health.go`.
- `internal/parser` reports diagnostics for every source through `parser.Report`: each required probe names the CSS selector that must match, and a parse that finds nothing, shrinks suspiciously or falls back keeps the previous cache entry instead of overwriting it. The sink is wired in `cmd/bot/main.go` into `health.Tracker.ParserReport` (alerts `parser_layout` for missing selectors and `parser_guard` for a kept cache or a fallback path) and into the bot, which renders it in `/parserLogs`.
- Parser alerts are actionable: `notification.HealthAlertButtons` attaches the `parser_reparse` button to every parser alert, the `reparseCb` handler runs the parser out of schedule for admins, and `/parserhealth` renders the live snapshot from the bot's `healthSource` (wired as `health.Tracker` in `cmd/bot/main.go`).
- Calendar alerts are actionable too: the same button table attaches `calendar_sync` to `calendar_failures`/`calendar_stale`, and the `calendarSyncCb` handler runs the change sync plus the reconcile pass wired as `bot.SetCalendarSyncFunc` in `cmd/bot/main.go` — failed days are retried because `LastManualSyncedDay` only advances after a successful pass.
- API alerts are actionable as well: `notification.HealthAlertButtons` attaches `api_probe` to `api_errors`, and the `apiProbeCb` handler runs `internal/apiprobe` against the bot's own listener (`bot.SetAPIProbeFunc` in `cmd/bot/main.go`, base URL from `apiProbeBaseURL`) and replies with a status and response-time table. `health.APIEndpointStat` tracks requests, errors, last/average/maximum latency per endpoint and a `Slow` flag driven by `health.api_slow_ms`, so the `api_errors` detail carries a `slow:` line next to `paths:`.
- `health.IncidentLog` keeps the history of parser, calendar and API incidents in the chat DB (`bot_state`, key `health.incidents`): `HealthNotifier` records an alert when it fires and closes it on recovery, the button handlers call `bot.markIncidentFix` so the manual fix is kept as the outcome, and `/incidents` renders the last 20 of the stored 100 records on top of a live per-endpoint API slice from the tracker (requests, errors, average and maximum latency). Every record stores the `health.StartupStamp` it started under (build summary plus a SHA-256 prefix of the config file, `configStamp` in `cmd/bot/main.go`), so `IncidentLog.NoteStartup` marks an open API incident as a restart, a config edit or a deploy with the matching note from `ru.json`.
- The data-loss guard thresholds are config, not constants: `parser.guard.disabled`, `parser.guard.min_items` (default 10) and `parser.guard.max_drop_percent` (default 80) are handed to `parser.Guard`, and each trip reaches the admins as the `parser_guard` alert (`health.parser_guard_failures`, default 1 — the first trip).

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
- Parity surface: `go run ./scripts/paritycheck`; keyboard layouts: `go test ./internal/telegram -run Golden`; message texts: `go test ./internal/telegram ./internal/notification`
- Race detector: `go run ./scripts/racecheck` (needs `CGO_ENABLED=1` and a C compiler — gcc on Linux, clang on macOS, mingw-w64 on Windows; `-check` only reports)
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
