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
  - `archive/` — SQLite repository for timetable archive (modernc.org/sqlite, pure Go); `migrations/*.sql` is embedded and applied by `Repository.EnsureSchema` from `New`, and `retention.go` owns the day cutoff, the scoped `timetable_archive` delete and the compaction the backup job calls.
  - `cache/` — file-backed RaspCache with in-memory state, hit/miss metrics and the event bus drained by the notifier.
  - `telegram/` — telego bot. `bot.go` registers everything and declares the `archiveStore` interface the bot depends on; `chat.go` + `chat_queries.go` + `chat_subscriptions.go` hold the chat database; `commands.go`, `settings_text.go`, `callbacks.go`, `menus.go`, `weekcontrol.go` carry the user surface; `timetabledays.go` is the single owner of day selection (the `daySource` choice between cache and archive, the group/teacher target of a chat, `daysToMaps`) and `texthints.go` holds the small label helpers; one file per feature for the rest (`history.go`, `stats.go`, `archive.go`, `ics.go`, `calls_edit.go`, `calls_settings.go`, `calls_campus.go`, `compare_groups.go`, `test_subscriptions.go`, `debug_commands.go`, `google_calendar.go`, `google_store.go`, `admin.go`, `admin_commands.go`).
  - `api/` — gin REST API: info, groups, teachers, group/teacher by name, parser-health, health, Google OAuth callback; `metrics.go` owns the optional Prometheus route (`Server.SetMetricsConfig`/`Server.SetMetricsSources`), which stays outside the `/api` key gate and serves aggregate counters only, and declares the metric surface in one place through `MetricFamilies()` and `MetricLabelNames()`.
  - `apiprobe/` — live probe of the bot's own API endpoints (targets built from the cache, statuses, response times, health summary) used by the `api_errors` alert button.
  - `health/` — metrics tracker (parser, calendar sync, API) with threshold-based alert evaluation; `state.go` persists it through the `StateStore` interface, and `telegram.go` holds the bounded Telegram counters (commands, callbacks, notifications, send transports) that both the bot and the notification scheduler feed.
  - `google/` — Google Calendar OAuth2 user client, Service Account sync, lesson event builders.
  - `image/` — fogleman/gg timetable PNG renderer that keeps the old TypeScript layout: a 30px centered heading, two day tables per row, an 18px day header over a 14px column header, multiline cells that grow their row, and the 8px footer with the Viber/TG/VK links, the generation stamp and the credit line; the college logo is embedded from `mgke.png`.
  - `calendar/` — ICS calendar export (hand-rolled, no external lib).
  - `notification/` — cron scheduler (robfig/cron), event-to-message mapping and health alert dispatch.
  - `formatter/` — output formats: default, compact, visual, litolax.
  - `parity/` — TS ↔ Go surface comparator used by `scripts/paritycheck` and the offline parity test.
  - `preflight/` — pre-deploy checks: config keys, credentials, the webhook configuration, locale keys, storage, the metrics route (`metrics.go` serves it on an ephemeral loopback port and parses the exposition with `expfmt`), the live site (dates, calls — `checkCalls` also warns when the page's `date-updated` is older than `-calls-stale-days`, 180 by default, or lies in the future), a rendered PNG; `scripts/preflight` prints the report.
  - `testgolden/` — golden-text normalization (dates, week numbers, callback data) shared by the message golden tests; tests only.
  - `tgstub/` — stub Telegram Bot API: records every bot call and answers `setWebhook`/`deleteWebhook`/`getWebhookInfo`/`getUpdates` (holding the request for its timeout, waking as soon as an update is queued)/`sendMessage`/`editMessageText` like Telegram does, an edit echoing the message id it was given; `LoopbackBaseURL` refuses a non-loopback listen address; the `container` CI job runs both transports against it and `scripts/devbot` runs local development against it.
  - `backup/` — online SQLite snapshot (`VACUUM INTO`), strictly scoped backup-directory retention and the cron entry that runs both.
  - `sqlitedb/` — the single owner of SQLite connections (`Open`): pins the pool to one connection, puts the configured `busy_timeout` on every connection it opens, verifies the journal mode the database actually reports (a silent fallback from WAL is a startup error), and `Retry` repeats an idempotent write that hit `SQLITE_BUSY`, reporting each retry and a final failure through the caller's callbacks.
  - `timetable/` — typed day/lesson domain (`Day`, `Lesson`, `Week`), the `дд.мм.гггг` date helpers and the raw-to-typed converters every consumer shares.
  - `reminder/` — picks the running and the next lesson from a day plus the bell schedule, and computes the jittered reminder moment.
  - `rooms/` — canonical room keys (`Key`, idempotent normalisation) and the free-room set for a pair.
  - `schedulediff/` — day-to-day change detection: added, removed, moved, room/teacher/time changed.
  - `search/` — fuzzy group and teacher search with transliteration and per-chat aliases.
  - `utils/` — academic week index, subject list.
- `docs/` — user-facing instructions (Google Calendar). `scripts/paritycheck/` — parity checker binary; `scripts/preflight/` — pre-deploy check; `scripts/monitoringcheck/` — offline check of the `deploy/monitoring` set (`internal/preflight/monitoring.go` parses the rules and the Alertmanager config, compares the Kubernetes copies with the plain files, validates the dashboard import model, requires every referenced `mgke_*` name to be declared and rendered and the other way round, and runs `promtool check rules`/`promtool test rules` when promtool is installed — a missing promtool is an explicit skip with exit code 1, never a pass); `scripts/racecheck/` — local race-detector runner that checks the cgo/C-compiler prerequisites first (`internal/racecheck` holds the plan logic); `scripts/tgstub/` — the stub Bot API server the container smoke test starts (`internal/tgstub` holds the handler); `scripts/devbot/` — the offline development profile: serves the stub on loopback, overrides the token and `api_base_url`, then runs `internal/app` against it with `configs/config.dev.yaml`.
- `README.md` / `README.en.md` — mirrored documentation, kept in sync by `tests/docs_test.go`.
- `Dockerfile`, `docker-compose.yml`, `.dockerignore` — container build; the builder tag comes from the `GO_VERSION` build argument, so every compose file passes it through from the environment and a local build exports it from `go.mod` first; runtime config comes from env vars, state lives in the `/data` volume.
- `deploy/monitoring/` — the monitored contract of the `/metrics` route, ready to load without editing: `prometheus/rules/mgke.yml` (the alert rules) with `prometheus/rules/mgke.test.yml` (their promtool unit tests) next to it, `prometheus/prometheus.yml` (scrape config with the target and the token coming from the volumes the stack writes), `alertmanager/alertmanager.yml` (the profile with empty receivers), `grafana/dashboards/mgke-timetablebot.json` (the dashboard in the Grafana import model) with `grafana/provisioning/`, `kubernetes/` (ServiceMonitor, PrometheusRule and the dashboard ConfigMap) and `alloy/config.alloy` (the Grafana Cloud case); `docker-compose.monitoring.yml` runs Prometheus, Grafana and, under the `alerts` profile, Alertmanager. `tests/monitoring_test.go` holds the copies together, refuses a metric name `api.MetricFamilies()` does not declare and checks the JSON shape hedge by hedge; the preflight `metrics` check refuses a `mgke_*` name the scrape does not serve.
- `docker-compose.webhook.yml` + `deploy/caddy/Caddyfile` and `docker-compose.webhook-nginx.yml` + `deploy/nginx/default.conf.template` — the two webhook-mode deploys: the reverse proxy terminates TLS with an automatically renewed Let's Encrypt certificate, forwards only the webhook path to the bot's listener and answers `404` everywhere else. `tests/deploy_test.go` checks the compose invariants and `internal/telegram/webhook_test.go` pins both examples to the default webhook path and listen port, so renaming either in the code cannot leave the examples broken.
- `.github/workflows/ci.yml` — quality gates on Linux, Windows and macOS; `.github/workflows/release.yml` — releases and the GHCR image; `.github/workflows/security.yml` — the scheduled `govulncheck` scan; `.github/dependabot.yml` — weekly dependency bumps.

## Architecture Overview (Flow)
- External inputs arrive via Telegram bot (long polling) or HTTP API.
- Bot commands are routed in `internal/telegram/bot.go` via command name or text matching; messages landing in an input scene go through the scene registry in `internal/telegram/scenes.go`.
- `Bot.Run` picks the update transport between long polling (the default) and the webhook in `internal/telegram/webhook.go`; the whole contract — validation, listener, statuses, shutdown, observability and tests — is in the “Webhook Transport” section below.
- Parser fetches HTML from the college site, normalizes via goquery, emits events into `cache/`.
- `cmd/bot/main.go` drains those events after every parse into `internal/notification`, then syncs only the changed days into Google Calendar and finally reconciles calendars that fell behind.
- `internal/health` records parser, calendar and API outcomes; the scheduler polls it and messages admins while an alert is active.
- Parsed data is stored in `cache/` (file-backed JSON in `cache/rasp/`) and `archive/` (SQLite).
- The site publishes several weeks at once, one content block per week: `internal/parser` walks every block (`findScopes`/`eachTable`), never the first one only, and merges the days of a group/teacher by date (`mergeDays`), so a repeated date is overwritten by the later block and the cache always holds every published week.
- Every cached day carries a plain `дд.мм.гггг` date in its `day` field: the formatters, the archive index, the notification bus and the Google/ICS exporters all parse it with `time.Parse("02.01.2006", ...)`. `buildDayColumns` only accepts a header cell whose text holds a parseable date, counts a bare weekday header as an undated column, skips it and reports it through `parser.Report` (the group and teacher parsers sum those counts per parse and turn them into inverted probes, so a partially undated page raises `parser_layout` instead of only a log warning), and the parser reports a required `th[colspan] with dd.MM.yyyy` probe so a page without parseable dates raises it too. `cache.New` normalizes legacy labels ("Понедельник, 31.08.2026") on load and drops a day it still cannot date — in the typed schedules and in the raw view `SetGroups`/`SetTeachers` merge — so no cached day lacks a date.
- Telegram messages: a command or a text button always sends a new message; only an inline button edits the message it belongs to, using the message id of the callback query (`Update.MessageID`). Never route an edit through a message id stored in the chat row.
- Timetable days are fetched only through `internal/telegram/timetabledays.go`: callers pass an explicit `daySource` (`daysFromCache` for the day/week keyboard view, `daysFromArchive` for the navigation callbacks, `archiveDaysForRange` for `/archive` and `/history`, which must not fall back to the cache). The archive is reached through the `archiveStore` interface — never through a type assertion on `Bot.archive`, so `NewBot` accepts any implementation and `nil` means "no archive".
- Output is delivered through telego (Telegram) or gin (HTTP API).

## Where to Add New Code
- New bot command: add struct either to `internal/telegram/commands.go` (user surface) or to the feature file it belongs to, implement `Command` interface, register in `registerAll()`. Do not create a catch-all file: one feature, one file.
- New callback: add struct in `internal/telegram/callbacks.go`, implement `Callback` interface, register in `registerAll()`.
- New menu: add one spec to the table in `internal/telegram/menus.go`; do not hand-register its command or text handlers.
- New input scene: add a typed scene constant and a route in `internal/telegram/scenes.go`; never compare raw scene strings in handlers.
- New API endpoint: add handler in `internal/api/server.go`, register route in `routes()`; both README files must document it (`tests/docs_test.go`). The Prometheus route is the exception: it is registered by `Server.SetMetricsConfig` from `internal/api/metrics.go`, never from `routes()`, because it must not sit behind the `/api` key middleware.
- New metric on `/metrics`: add the collector in `internal/api/metrics.go` (`renderMetrics`/`renderSourceMetrics`) and declare its name in `metricFamilies`; when it needs state from another package, extend `MetricsSources` with a `func() int`/`func() (T, error)` accessor wired in `internal/app/app.go`. Never put an identifier, a token or a label free-form string into the output: values are numbers and the label names are the fixed `MetricLabelNames()` set. The preflight `metrics` probe wires the sources the configuration can supply (the archive once `db_path` exists, the webhook block, and the scheduler/reminder gauges while `telegram.noticer` is on) and `excusedMetricFamilies` excuses a family only while its source is genuinely absent, so a configured source that stops rendering fails the check.
- New counter key (a command, a callback prefix, a notification kind, a transport): add the constant next to the send path that reports it and count it at that send path — never at a distance from it. `health.TelegramCounters` normalises the key (`sanitizeCounterKey`, `a-z0-9_-`, 64 characters), drops empty keys and caps each dictionary at 128 keys with the excess in `other`, so a label can never carry user text.
- New alert rule, dashboard panel or scrape asset: keep it inside `deploy/monitoring/` so the preflight `metrics` check, `go run ./scripts/monitoringcheck` and `tests/monitoring_test.go` see it; a rule belongs in `prometheus/rules/mgke.yml` with its expectation in `mgke.test.yml` (annotations stay template-free so promtool can compare them verbatim), a panel in `grafana/dashboards/mgke-timetablebot.json`, and the Kubernetes copies are regenerated by indenting the plain files under the custom resource (the test prints the command when they drift). The parser staleness alerts must combine the lag gauge with `mgke_cache_groups_last_update_seconds`, because a parser that never ran reports a lag of zero. Run the same checker after the edit — the `monitoring` CI job runs that very command, so a name the route does not serve or a copy left behind fails the pull request.
- New metric or alert: record it in `internal/health` and map the alert key to a message in `internal/notification/health.go`; a new API route worth probing goes into `apiprobe.Targets` in `internal/apiprobe/probe.go`.
- New config field: add it with a `yaml` tag — the `MGKE_*` environment name is derived from that tag automatically; document it in `configs/config.example.yaml` and in both README files.
- New parser source: add a file in `internal/parser/` that returns a `Report` from `report.go` — every required probe names the selector it expects, a source that carries dates reports a required date probe, and every row or column the parser drops is counted through `probeSkipped` (see “Parser Diagnostics Contract”).
- New end-to-end case: extend `tests/e2e_test.go` (real SQLite chat database) or `internal/telegram/*_e2e_test.go` (bot surface) — do not add a mock-only test where a real repository works.
- New pre-deploy check: add a `Check` to `internal/preflight` and cover it offline with an `httptest` fixture; the command must stay usable without network (`-skip-site`). A check that starts a server of its own (like `checkMetrics`) listens on `127.0.0.1:0` and shuts down through a context, so it never races the bot for a port.
- New user-facing message: add it to the matching golden scenario (`internal/telegram/messages_golden_test.go` or `internal/notification/messages_golden_test.go`) in the same change.
- New locale string: add key to `internal/i18n/locales/ru.json`, use `b.loc("key")` in code.

## Build, Test, and Development Commands
- Runtime: Go 1.27.3+ — the minimum is pinned in `go.mod` (`go 1.27.3`); CI reads it through `go-version-file: go.mod`, the container build receives the same version through the `GO_VERSION` build argument, and `tests/goversion_test.go` keeps `go.mod` the only file in the tree that carries a version literal.
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
- `go run ./scripts/tgstub -listen 127.0.0.1:18090` — the stub Telegram Bot API the `container` CI job talks to; `GET /_stub/calls` lists every call the bot made, `GET /_stub/health` is the readiness probe, `POST /_stub/updates` queues an incoming update.
- `go run ./scripts/devbot` — offline development profile: the stub plus the real bot, with the token and `telegram.api_base_url` overridden before the config loads and state under `./.dev/`; `-command start` queues `/start`, `-send` queues arbitrary text.
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` — lint the workflow files after touching `.github/workflows/`.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — dependency vulnerability scan (the scheduled `security.yml` job).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/bot/` — cross-compilation check; repeat for `linux/arm64` and `darwin/arm64` after touching platform-specific code.

## CI and Releases
- `.github/workflows/ci.yml` runs on every push to `main` and on every pull request. Independent jobs: `quality` (cross-platform build, vet and the tool binaries; gofmt and the release-target cross-compilation only on Linux), `test` (suite plus `coverage.out` artifact), `newest-toolchain` (the suite on the newest stable Go), `race` (`go run ./scripts/racecheck` — `-race` over the whole suite), `fuzz` (20 s per fuzz target over the pure libraries, the parsers, the counters and the webhook), `parity` (surface vs the `old` branch, golden keyboard layouts, docs guard), `monitoring` (`go run ./scripts/monitoringcheck -strict` plus `promtool test rules`, with promtool installed from a release archive pinned by version and SHA-256 in the workflow env), `container` (image build, container start, `/api/health` 200, build metadata, the container timezone, the graceful `SIGTERM` shutdown — taken only once the long polling loop is confirmed running — and a second container run in webhook mode, with both runs talking to the stub Bot API instead of `api.telegram.org`), `devbot` (the offline profile without Docker: `scripts/devbot -command ping` against the loopback stub, `pong` in `/_stub/calls`, no `api.telegram.org` in the log and a graceful `SIGTERM`) and `workflow-lint` (`actionlint`).
- The platform-sensitive jobs (`quality`, `test`, `newest-toolchain`, `race`) carry a matrix over `ubuntu-latest`, `windows-latest` and `macos-latest` with `fail-fast: false`, and every workflow sets `defaults: run: shell: bash` so one command string works on all three. The platform-neutral gates (`fuzz`, `parity`, `monitoring`, `container`, `devbot`, `workflow-lint`) stay on Linux because they need Docker, a `git fetch` of the `old` branch, `jq` or the Linux promtool archive, or because a fuzzing run only makes sense on a fast throwaway runner — `go test -fuzz` is reproducible on any machine with the same `-fuzztime`.
- Coverage is measured on all three operating systems but only the Linux run uploads the artifact and the summary, otherwise the artifact names would clash. The race job probes with `racecheck -check` first and, when Windows reports the detector as unavailable, warns instead of failing — `-race` builds through cgo and the Windows runner has no guaranteed C toolchain.
- `.github/workflows/security.yml` runs `govulncheck` on push to `main`, on a weekly schedule and on demand; it is deliberately outside CI so an advisory never blocks a pull request.
- Actions are pinned by commit SHA with the release tag in a comment; Dependabot bumps the SHA and the comment together, so keep the `# vN` comment when editing a `uses:` line.
- Every job sets `GOTOOLCHAIN: local`, so the pinned `go 1.27.3` from `go.mod` is the version the checks pass with and a newer toolchain is never substituted silently. The `newest-toolchain` job opts in explicitly with `go-version: stable`, so a Go release that breaks the suite shows up without moving the pinned minimum.
- `.github/workflows/release.yml` first runs `verify` (build, vet, tests on Linux, Windows and macOS) on the same revision and only then publishes: a push to `main` pushes the multi-arch image to `ghcr.io/blindmaster24/mgketimetablebot` as `:edge`/`:main`; a `v*` tag additionally builds linux/amd64, linux/arm64, windows/amd64 and darwin/arm64 archives, attaches them to a GitHub Release and gives the image `:<major>.<minor>.<patch>`, `:<major>.<minor>` and `:latest`. Do not pin a concrete release number in the docs: link the releases page and the GHCR package instead.
- The release build injects the tag, commit and build date through `-X main.version`, `-X main.commit` and `-X main.date`; `cmd/bot/main.go` logs them at startup, `internal/build` carries them into `GET /api/health`, `GET /api/info` and the admin `/debug` command. Bump the version by tagging, never by editing code.
- `internal/image` finds a font by probing the known Debian/Alpine, macOS and Windows paths and then scanning the system font directories, keeping only fonts that actually load; the runtime image installs `font-dejavu` so the PNG rendering works in the container too.
- The container runs in the college timezone (`TZ=Europe/Minsk`, `tzdata` installed, `TZ` passed by `docker-compose.yml`). Day boundaries, the academic week index and the notification crons are all derived from `time.Local`, so a container in UTC would fire notifications three hours late — `tests/container_test.go` guards both files.
- The Dockerfile builds its stage on `$BUILDPLATFORM` and cross-compiles with `GOOS=$TARGETOS GOARCH=$TARGETARCH`, so the multi-arch image never compiles Go under QEMU. Its base image is `golang:${GO_VERSION}-alpine`, and both image jobs in the workflows fill `GO_VERSION` from `actions/setup-go` reading `go.mod`, so a release can never ship from a toolchain other than the pinned one. Keep `CGO_ENABLED=0` — the bot is pure Go and the runtime image has no libc for cgo.
- Add a new CI job only when it fails for a reason a developer can reproduce locally, and mirror every new local check in the Verification Checklist above.

## Verification Checklist
- `go vet ./...` for static analysis after any changes.
- `go test -count=1 -p 1 ./internal/... ./tests/...` before commit.
- `go build -o bot ./cmd/bot/` to verify binary compiles.
- `go run ./scripts/paritycheck` to verify the Telegram surface still matches the old TypeScript bot.
- `go run ./scripts/monitoringcheck -promtool /path/to/promtool -strict` after touching the monitoring assets: one pass parses the rules, the Alertmanager config and the dashboard, compares the Kubernetes copies, checks the metric names against `api.MetricFamilies()` and both directions of the render, and runs `promtool check rules` and `promtool test rules` (`promtool check rules deploy/monitoring/prometheus/rules/mgke.yml` and `promtool test rules deploy/monitoring/prometheus/rules/mgke.test.yml` alone are enough for a quick loop). Without promtool the script prints an explicit skip with `result: NOT VERIFIED` and exits 1, so a machine that cannot run the tool says so instead of reporting a pass — the `monitoring` CI job installs promtool 3.15.0 against its pinned SHA-256 and runs the same command, and `tests/monitoring_test.go` keeps gating the files offline (metric names, dashboard shape, Kubernetes parity, compose wiring, promtool expectations).
- `go run ./scripts/preflight -config configs/config.yaml` before a deploy: it parses the live site and checks dates, calls, credentials, the locale keys, storage, PNG rendering and the metrics route in one run — the `metrics` check serves the route on a loopback port, refuses a scrape that does not answer `401` without the token and fails when a family, its `HELP`/`TYPE` or a `mgke_*` name used under `deploy/monitoring` is missing (exit code 1 on a failure, `-strict` also fails on warnings).
- `go test -count=1 ./internal/app/ -run EndToEnd` after touching a parser diagnostic: it drives the real fetcher, health tracker, notifier, alert buttons and incident log, so a probe that only logs is caught here.
- `go test -count=1 ./tests/` to verify the documentation still matches the bot surface, API routes and config keys.
- `go build ./...`, `go vet ./...` and the suite on Windows and macOS as well (the CI matrix) — a change that only compiles on Linux reaches the platform-specific jobs red.
- `GOOS=<target> GOARCH=<arch> CGO_ENABLED=0 go build ./cmd/bot/` for the release targets after touching platform-specific code (`cmd/bot/signals_*.go` is the current example).
- `go test -run FuzzDiff -fuzz FuzzDiffKeepsChangesWithinTheDays -fuzztime 30s ./internal/schedulediff/` after touching a fuzzed package — the same command for `internal/search`, `internal/reminder`, `internal/rooms`, `internal/parser`, `internal/health` and the webhook target in `internal/telegram` (the `fuzz` job runs all of them for 20 s each).
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
- Telegram bot via telego — long polling by default, an optional webhook mode (`telegram.webhook`, secret token, own TLS or a reverse proxy) configured entirely from the config or environment.
- Commands: 68 total — schedule and setup (`/start`, `/help`, `/setup`, `/day`, `/week`, `/calls`, `/group`, `/teacher`, `/image`, `/cabinet`, `/history`, `/archive`, `/alias`, `/comparegroups`, `/groups`, `/teachers`, `/ics`, `/google_calendar`, …), admin (`/debug`, `/send`, `/trigger`, `/forceparse`, `/noticedebug`, `/sql`, `/restart`, …) and menu text pseudo-commands. The full list is in `README.md` and enforced by `scripts/paritycheck`.
- Keyboard buttons match command text via i18n keys.
- Parser (table-based) for groups, teachers and the bell schedule, with change detection.
- File-backed cache with JSON persistence in `cache/rasp/`.
- SQLite archive for historical schedule data.
- REST API (gin) with /api/info, /api/groups, /api/teachers, /api/group/:name, /api/teacher/:name, /api/parser-health, /api/health, plus the Google OAuth callback route. `/api/health` returns 503 while alerts are active.
- Optional Prometheus `/metrics` route (`metrics` config section, off by default): aggregate counters and gauges for the build, cache, parser, calendars, HTTP API, archive, scheduler, reminders, webhook state and Telegram traffic (commands, callbacks, notifications, send failures) — no secrets, tokens, chat IDs or message texts; can be gated by `metrics.token`.
- Monitoring set in `deploy/monitoring` (dashboard, 14 alert rules with promtool tests, compose stack, Prometheus Operator manifests, Grafana Alloy config for Grafana Cloud) plus `tests/monitoring_test.go`, which keeps every `mgke_*` name honest against `api.MetricFamilies()`.
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
- Thresholds live in the `health` config section (`disabled`, `check_minutes`, `parser_*`, `calendar_*`, `api_*`, `backup_failures`); add new alert keys to both `internal/health` and `internal/notification/health.go`.
- The archive maintenance reports into the tracker: `MaintenanceSuccess`/`MaintenanceFailure` drive the `backup_failed` alert (`health.backup_failures`, default 1) and its recovery, while `ArchiveBusyRetry`/`ArchiveBusyFailure` count the writes retried after a busy lock; both surface on `/metrics` and in `GET /api/health`. The message texts are covered by the `health/backup_failed` scenario in `internal/notification/messages_golden_test.go`.
- SQLite tuning is the `sqlite` section (`busy_timeout_ms`, `busy_retries`); every connection is opened through `internal/sqlitedb`, so a new database never calls `sql.Open` directly and the archive's journal mode is verified instead of assumed.
- `internal/parser` reports diagnostics for every source through `parser.Report`: each required probe names the CSS selector that must match, and a parse that finds nothing, shrinks suspiciously or falls back keeps the previous cache entry instead of overwriting it. The sink is wired in `cmd/bot/main.go` into `health.Tracker.ParserReport` (alerts `parser_layout` for missing selectors and skipped items, `parser_guard` for a kept cache or a fallback path) and into the bot, which renders it in `/parserLogs`.
- Parser alerts are actionable: `notification.HealthAlertButtons` attaches the `parser_reparse` button to every parser alert, the `reparseCb` handler runs the parser out of schedule for admins, and `/parserhealth` renders the live snapshot from the bot's `healthSource` (wired as `health.Tracker` in `cmd/bot/main.go`).
- The webhook transport is observable: `Bot.WebhookStatus` merges the configured endpoint with a live `getWebhookInfo` answer (and keeps the last known state when Telegram is unreachable), `/webhook` renders it for admins with a `webhook:reset` button that re-registers the webhook (`Bot.ResetWebhook`), and `internal/api` exposes the same snapshot through `Server.SetWebhookStatus`, wired in `cmd/bot/main.go` into the `webhook` block of `GET /api/info`.
- Calendar alerts are actionable too: the same button table attaches `calendar_sync` to `calendar_failures`/`calendar_stale`, and the `calendarSyncCb` handler runs the change sync plus the reconcile pass wired as `bot.SetCalendarSyncFunc` in `cmd/bot/main.go` — failed days are retried because `LastManualSyncedDay` only advances after a successful pass.
- API alerts are actionable as well: `notification.HealthAlertButtons` attaches `api_probe` to `api_errors`, and the `apiProbeCb` handler runs `internal/apiprobe` against the bot's own listener (`bot.SetAPIProbeFunc` in `cmd/bot/main.go`, base URL from `apiProbeBaseURL`) and replies with a status and response-time table. `health.APIEndpointStat` tracks requests, errors, last/average/maximum latency per endpoint and a `Slow` flag driven by `health.api_slow_ms`, so the `api_errors` detail carries a `slow:` line next to `paths:`.
- `health.IncidentLog` keeps the history of parser, calendar and API incidents in the chat DB (`bot_state`, key `health.incidents`): `HealthNotifier` records an alert when it fires and closes it on recovery, the button handlers call `bot.markIncidentFix` so the manual fix is kept as the outcome, and `/incidents` renders the last 20 of the stored 100 records on top of a live per-endpoint API slice from the tracker (requests, errors, average and maximum latency). Every record stores the `health.StartupStamp` it started under (build summary plus a SHA-256 prefix of the config file, `configStamp` in `cmd/bot/main.go`), so `IncidentLog.NoteStartup` marks an open API incident as a restart, a config edit or a deploy with the matching note from `ru.json`.
- `metrics` is a second, deliberately separate surface: `internal/api/metrics.go` renders the prometheus text format from the same `health.Snapshot` plus the `MetricsSources` accessors wired in `internal/app/app.go` (archive stats and day bounds, scheduler entries, tracked reminders, the mapped `Bot.WebhookStatus`), filters per-route API stats through `registeredRoutes()` so a route the server does not serve can never appear, and owns its own token check (`metrics.token`, constant-time, `401` with `WWW-Authenticate`). It exposes aggregates only — the webhook block is booleans and a count, never the endpoint or the secret — and `internal/api/metrics_test.go` asserts that through the real handler.
- The data-loss guard thresholds are config, not constants: `parser.guard.disabled`, `parser.guard.min_items` (default 10) and `parser.guard.max_drop_percent` (default 80) are handed to `parser.Guard`, and each trip reaches the admins as the `parser_guard` alert (`health.parser_guard_failures`, default 1 — the first trip).

## Parser Diagnostics Contract
A degraded page is never silent: it is a probe, and every probe reaches `health.Tracker.ParserReport` through `parserLayoutIssues`/`guardIssues` in `internal/app/wiring.go`.

- Two probe shapes exist. A normal probe fails when a required selector finds nothing (`Found == 0`); an inverted probe (`Inverted: true`, built with `probeSkipped(selector, expected, missing, warning)`) fails when the parser had to *skip* something, and carries the count in `Probe.Missing`. `Probe.OK()`, `Report.Failing()` and `Report.Summary()` (`skipped: <selector> (N skipped)`) understand both.
- A third shape, `Advisory: true` (built with `probeAdvisory`), never fails: `Probe.OK()` is true and `Report.Advisories()` lists the ones that found nothing. It exists for a condition that must stay visible without alarming — a page the site simply does not publish data for. `parserLayoutIssues` forwards it as `health.LayoutIssue{Advisory: true}`, which `health.Tracker.ParserReport` stores for the snapshot but never counts as a failure (`hasAlertingIssue`), so `/parserhealth`, `/parserlogs` and `GET /api/health` show it while the snapshot's alert list stays empty and `parser_layout` stays silent; `layoutDetail` skips advisory entries, and the preflight `probeHints` turns them into `warning:` hints.
- `health.LayoutIssue.Missing` carries that count into the alert detail (`skipped N`) and into `GET /api/health`; `layoutDetail` prefixes the detail with `missing` when a selector vanished and with `skipped` when only items were dropped. No new alert key, metric or monitoring rule is needed for a new degradation — it rides `parser_layout` and `health.parser_layout_failures` (default 2 runs).
- A `Report.Warnings` entry is for `/parserLogs` only and never alerts; a condition admins must hear about belongs in a probe, with the same text as the warning so both surfaces agree.
- Group and teacher parsers: `th: day header without dd.MM.yyyy` and `th: day header with an impossible date` count the columns `buildDayColumns` dropped.
- Bell schedule: `td: pair label` counts slot rows whose pair label cannot be read and `tr: two time ranges` counts rows that carry times but no valid slot. A shift header (`<th colspan="2">1 смена</th>`) is not a slot row and is never counted.
- The bell-schedule page date (`[date-updated] with dd.MM.yyyy`, with a `дд.мм.гггг` in the page heading as a fallback) is **advisory**: the college's live page carries no date at all, so a required probe would leave `parser_layout` firing on every run and `GET /api/health` answering 503 forever. The admin surface says it instead — `/parserhealth` renders `parser_health_layout_advisory` plus `parser_health_calls_parsed_at`, `/parserlogs` the `parser_logs_advisory_prefix` line, the calls settings menu and the refresh reply say `call_site_date_missing` followed by `call_site_parsed_at`. That parse moment is the whole fallback for freshness: `cache.SetCallsNotify` records when the bot stored the site schedule in `Calls.Site.UpdatedAt`, and `SetCallsSiteUpdatedAt` only overwrites it when the page actually carries a date, so `Bot.siteCallsParsedAt` (raw empty, stamp set) can show it without pretending it is the site's own date. Preflight warns about a missing or stale date.
- The preflight `site:*` checks turn a failing probe into an `error:` hint and an advisory one into a `warning:` hint, and `site:calls` warns (not fails) when slots parsed but the date or a label did not, so `-strict` still fails a deploy.
- `TestUndatedDayColumnReachesTheAdminsEndToEnd` in `internal/app/app_test.go` proves the alerting chain — parser → report → tracker → notifier → admin message and incident log — and `TestUndatedBellScheduleStaysVisibleWithoutAnAlert` proves the advisory one: the missing page date lands in `tracker.Snapshot().Parser.Layout` marked advisory, raises no alert, sends nothing and creates no incident. A unit test that only checks `Report.Warnings` proves neither.

## Webhook Transport
Long polling is the default; `telegram.webhook` makes Telegram push updates to an HTTPS address instead. The transport is one file, `internal/telegram/webhook.go`, and `Bot.Run` only chooses between it and `runPolling`.

### Configuration and validation
- `webhookSettingsFrom` is the single validator: `ValidateWebhook` (preflight), `Bot.Run`, `Bot.WebhookStatus` and `Bot.ResetWebhook` all go through it, so a rule can never hold on one path and not on another.
- Rules: `url` is required and must be `https`; the served path comes from `url` when it has one, otherwise from `path` (default `/telegram/webhook`); `secret_token` accepts `A-Z a-z 0-9 _ -` up to 256 characters; `certificate` and `key` must be set together and must exist; `ip_address` must parse; `max_connections` is 1–100 and `buffer` 1–10000 (defaults 40 and 128) — a config typo must not turn into a multi-gigabyte channel allocation; `listen` defaults to `127.0.0.1:8082`.
- The path is validated by handing it to `http.ServeMux.HandleFunc` behind a recover (`validateWebhookPath`), so a path the mux cannot serve (`/telegram webhook`, `/telegram/{`) is a startup error with a clear message instead of the panic `ServeMux` raises — that panic used to escape `main` *after* `setWebhook`, leaving Telegram posting into a dead process.
- `telegram.api_base_url` (empty by default) points the whole bot at another Bot API server through `telego.WithAPIServer`; `config.Config.TelegramAPIServer` validates the address once and `NewBot` fails on a broken one. It exists for real deployments behind a local bot-api and for the container smoke test, which needs a stub Telegram to talk to.

### The listener
- telego deliberately owns no HTTP server: `telego.WebhookHandler` receives the raw JSON and its documentation puts the request authentication and the `context.WithoutCancel` wrapping on the caller. The transport keeps its own `http.Server` for exactly that contract — read-header 10 s, read/write 30 s, idle 60 s, a 1 MB body cap — not a parallel HTTP stack.
- Status table: a non-POST request gets `405` with `Allow`, a missing or foreign `X-Telegram-Bot-Api-Secret-Token` `401` (constant-time compare), an oversized body `413`, a handler failure `500` so Telegram retries. The mux answers `404` for anything else.
- `telegram.webhook.buffer` sizes the telego update channel and is the backpressure knob: while it is full the handler parks the POST until Telegram gives up and redelivers, which is why the buffer and the write timeout belong to the same story.
- Registration happens inside `UpdatesViaWebhook` through `telego.WithWebhookSet(ctx, params)`, so `setWebhook` (certificate included when configured) and the listener start as one step.

### Shutdown
- Every stop signal cancels the single root context of `cmd/bot/main.go` — `cmd/bot/signals_unix.go` (`SIGINT`, `SIGTERM`, `SIGHUP`, `SIGQUIT`), `cmd/bot/signals_windows.go` (`os.Interrupt`, `SIGTERM`), with a second signal exiting at once — so the transport stops on any of them, not on `SIGTERM` alone.
- `runWebhook` selects on the server error and on the update loop for the whole run and waits for `http.Server.Shutdown` (`webhookShutdownTimeout`, 5 s) before it returns, so `Bot.Run` returns with the listener already closed. That is what makes the closing `shutdown complete` line truthful for `stop_grace_period: 30s` and for the `container` CI job that greps for it. A fatal `Serve` error (an unreadable certificate fails immediately) is reported rather than dropped by a single non-blocking check.
- `runPolling` deletes a stale webhook first, so switching modes never hits the Telegram `409 Conflict`.

### Observability and the admin surface
- `Bot.WebhookStatus` merges the configured values with a live `getWebhookInfo` (3 s timeout) and keeps the last known answer when Telegram is unreachable; the secret and the certificate are reported as booleans, never as values.
- `/webhook` (`webhookCmd`, admin-only) renders the snapshot and offers `webhook:reset` (`webhookResetCb` → `Bot.ResetWebhook`), `GET /api/info` carries the same snapshot in its `webhook` block through `Server.SetWebhookStatus`, and `logWebhookInfo` logs the endpoint, the pending count and the last delivery error at startup. The alert side of this is in “Health Metrics and Alerts”.

### Deploy examples
- `docker-compose.webhook.yml` + `deploy/caddy/Caddyfile` (Caddy terminates TLS, issuance and renewal fully automatic) and `docker-compose.webhook-nginx.yml` + `deploy/nginx/default.conf.template` (nginx renders with `envsubst`, certbot issues and renews through the webroot). Both keep the HTTP API on the host loopback, expose only the webhook path and pass `X-Telegram-Bot-Api-Secret-Token` through unchanged.

### Tests
- `webhook_test.go`: `TestWebhookSettingsRejectBrokenConfiguration`, `TestWebhookSettingsApplyDefaultsAndDeriveEndpoint`, `TestWebhookSettingsKeepExplicitPathAndOptions`, `TestWebhookSettingsComeFromTheEnvironment`, `TestWebhookSetParamsCarryTheCertificate` and `TestRunReportsAnUnservableWebhookPathInsteadOfPanicking` pin the validation rules, the defaults, the endpoint derivation, the `MGKE_TELEGRAM_WEBHOOK_*` overrides, the certificate upload and the mux-pattern guard.
- `webhook_test.go`: `TestWebhookHandlerAcceptsOnlySignedRequests`, `TestWebhookHandlerRejectsOversizedBody`, `TestWebhookHandlerAsksTelegramToRetryOnFailure` and `TestWebhookHandlerKeepsTheUpdateContextAlive` pin the status table and the `context.WithoutCancel` contract; `FuzzWebhookHandlerKeepsStatusesSane` keeps any input inside that table.
- `webhook_status_test.go`: the status snapshot, its cached fallback when `getWebhookInfo` fails, long-polling mode, the admin gate on `/webhook` and the reset callback (including a foreign payload).
- `fakeapi_test.go`: `TestBotOverRealHTTPWebhookTransportStopsWithTheContext` runs the real `Bot.Run` over a real socket (the registered secret, `401` for a foreign one, `200` and a reply for a signed POST, a clean stop and a refused port), and `TestWebhookTransportDrainsTheInFlightUpdateBeforeStopping` keeps a partially sent POST in flight and fails if `Run` returns before the drain; `TestBotOverRealHTTPWebhookReplies` and `TestWebhookDeliversUpdatesToTheBot` cover the update path into the dispatcher, `TestRunDropsAStaleWebhookBeforePolling` the polling switch.
- `fakeapi_test.go`: `TestWebhookDeliversACallbackQueryToTheInlineEditPath` posts `internal/telegram/testdata/webhook_callback_query.json` — the payload the `container` job posts verbatim — through a real `Bot.Run` socket and fails unless the inline button answered its `callback_query`, the bot edited the message id that update carried (never an id from the chat row) and the edit targeted the chat of the update.
- `webhook_test.go`: `TestDeployExamplesRouteTheDefaultWebhookTarget` ties both deploy examples to `webhookDefaultPath` and `webhookDefaultListen`; `tests/deploy_test.go` checks the compose invariants, and `internal/api/ts_api_test.go` plus `internal/preflight/preflight_test.go` cover the `/api/info` block and the preflight check.
- `internal/tgstub/stub_test.go` drives the stub through the real telego client (`WithAPIServer`), so the recording surface the CI job relies on cannot rot — `editMessageText` echoes the message id it was given, a callback query is answered, a fresh `sendMessage` still reports message `1`, `getUpdates` holds the request for its timeout and a queued update wakes it (served once, and an empty poll answers an empty list rather than `null`), and a real long polling loop records its `getUpdates` call and stops with the context; `internal/telegram/apiserver_test.go` pins `telegram.api_base_url` — a broken address fails `NewBot`, a valid one makes the bot call the configured server.
- The long polling run of the `container` job waits for the `bot started, listening for updates via long polling` line before `docker stop` — the same literal `runPolling` logs — and `tests/tooling_test.go` keeps the workflow gate and that literal together, so the stop can never race the startup; `consumeUpdates` logs `stop signal received` whenever the root context is cancelled, including when the closed update channel wins the select.
- The `container` CI job runs the transport twice: long polling first, then a second container in webhook mode; both start on `--network host` with `MGKE_TELEGRAM_API_BASE_URL` pointing at `scripts/tgstub`, so no CI step ever reaches `api.telegram.org`, and the webhook run must register the endpoint with the secret, answer `405`/`401`/`200` on the live socket, deliver a signed update that comes back as a `sendMessage` call on the stub, deliver the signed callback query of `internal/telegram/testdata/webhook_callback_query.json` that must come back as an `answerCallbackQuery` plus an `editMessageText` of the message id the update carried, and still log `shutdown complete` after `docker stop --time`.
- Known gap: the race detector runs in CI only — the shutdown and drain paths are covered there, not locally on a machine without a C toolchain.

## Dependency Decisions
These are settled; do not reopen them without the user asking.

- `gin` stays the REST API router. `internal/api/server.go` keeps its routes, the `/api` key middleware and the panic recovery on gin, because rewriting them on `net/http` buys nothing.
- `fogleman/gg` stays the timetable PNG renderer: it is pure Go without CGO, and the font lookup in `internal/image` is built around it.
- `lib/pq` is used only by the one-off `cmd/migrate-pg`, which moves chats and subscriptions from PostgreSQL to SQLite. The bot itself runs on `modernc.org/sqlite`; no new `lib/pq` use may be added.

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
- HTTP-level bot tests talk real HTTP to a fake Telegram Bot API (`internal/telegram/fakeapi_test.go`) instead of stubbing the caller: use it for a transport change or a reply path.
- Fuzz targets live in `internal/parser/fuzz_test.go` (seeded with the real page snapshots), `internal/telegram/webhook_test.go` and, for the pure libraries, `internal/schedulediff/fuzz_test.go`, `internal/search/fuzz_test.go`, `internal/reminder/fuzz_test.go`, `internal/rooms/fuzz_test.go` and `internal/health/telegram_test.go`: `go test -run FuzzGroup -fuzz FuzzGroupParserStaysStableAndClean -fuzztime 30s ./internal/parser/`; the `fuzz` CI job runs every target for 20 s. A crasher is written to `testdata/fuzz/<target>/` and is kept as a permanent seed, so it runs on every plain `go test`.
- Fix test-first: write the failing test, watch it fail for the right reason, then fix. A defect found by fuzzing keeps its corpus file as the regression test.
- Never assign the process-global `time.Local`: every goroutine that calls `time.Now()` reads it (a leftover fasthttp dialer from an HTTP-level test, a cron tick, a notification cron), so the write is a real data race that fails the `race` job. Inject the clock instead — `Bot.SetNow` and `notification.EventNotifier.SetNow` — and pass an explicit `time.Time` into the date helpers (`getDayPhrase`, `todayDayOf`, `futureDays`); `tests/timezone_test.go` fails the suite if an assignment comes back. The timezone behaviour stays covered by `TestDayNoticeWorksInEveryTimeZone`, `TestSchedulerBindsTheCronToTheProcessTimezone` and the `TZ=... go test` runs.
- Use `httptest` for API handler tests.
- A diagnostic that should reach admins is tested end-to-end, not only at the unit: `internal/app/app_test.go` builds the real fetcher with the production `parserLayoutIssues`/`guardIssues` wiring, the real `health.Tracker`, the real `notification.HealthNotifier` and a `recordingSender`, parses a degraded page, and asserts the message text, the alert button and the incident record (`TestUndatedDayColumnReachesTheAdminsEndToEnd`, `TestUnparseableCallsHeaderReachesTheAdminsEndToEnd`). A unit test that only checks `Report.Warnings` does not prove the alert path.
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
- Fuzz targets: `go test -run Fuzz -fuzz <Target> -fuzztime 30s ./internal/<pkg>/` for `schedulediff`, `search`, `reminder`, `rooms`, `parser` and the webhook target in `telegram`
- Static analysis: `go vet ./...`

## Documentation Is Part of the Change
Documentation is not a follow-up task; it is part of the same commit and the same pull request.

- Any change that alters user-visible behaviour updates `README.md` and `README.en.md` together — they are mirrored, and `tests/docs_test.go` fails when a command, API route or config key appears in only one of them.
- Any change that alters developer-facing behaviour updates this file (`AGENTS.md`) in the same commit: the architecture bullet, the “Where to Add New Code” entry and the “Verification Checklist” that the change touches.
- A new config key goes into `configs/config.example.yaml` and both README files; a removed key leaves no trace in either.
- A new or changed metric, alert rule, dashboard panel or scrape asset updates `deploy/monitoring/` and the preflight expectations; a new alert key is documented in both READMEs next to its threshold.
- A new user-facing text updates its golden scenario (`internal/telegram/messages_golden_test.go`, `internal/notification/messages_golden_test.go`) and the locale file `internal/i18n/locales/ru.json`; a new Russian string never lands as an inline literal in Go.
- A new parser diagnostic, admin command, API route, config field, environment variable or CI job is documented where it lives: `/parserlogs` diagnostics in the parser sections, commands in the command table, routes in the API section, environment names in the `MGKE_*` table.
- If a change touches several of these, update all of them — a partially documented change is an unfinished change, and a pull request states which docs it touched.

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
