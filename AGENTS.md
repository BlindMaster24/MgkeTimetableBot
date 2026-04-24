# Repository Guidelines

## Project Structure & Module Organization
- Entrypoints: `src/bootstrap.ts` is runtime wrapper for start; `src/index.ts` bootstraps the app; `src/app.ts` registers services and starts them (DB sync, then `service.run()`).
- Data & models: `src/db/` configures Sequelize and cleanup; domain models live in `src/services/timetable/models/`.
- Services (`src/services/`):
  - `bots/` is the primary chat layer. Platform adapters: `tg/`, `vk/`, `viber/`. Shared abstractions: `bots/abstract/`. Commands live in `bots/commands/` (grouped by domain like `rasp/`, `settings/`, `admin/`), callbacks in `bots/callbacks/`, keyboards in `bots/keyboard/`, and event routing in `bots/events/`.
  - `parser/` fetches and caches timetable data (`parser/raspCache.ts`, `parser/types/`).
  - `timetable/` builds domain objects used by bot commands and APIs.
  - `api/` serves HTTP endpoints with handlers under `api/methods/`.
  - `alice/` and `vk_app/` implement voice/VK app flows; `google/` syncs with Google Calendar (`google/api/`).
  - `image/` renders timetable images and handles cleanup.
- Utilities: `src/utils/` has shared helpers (time, http, queues, serialization, random, regex, arrays). `src/formatter/` contains formatting strategies with a shared abstract base.
- Other folders: `tests/` for TS test scripts; `scripts/` for one-off helpers; `public/` for static assets.

## Architecture Overview (Flow)
- External inputs arrive via bot platforms or HTTP API.
- Bot events are routed in `src/services/bots/*`, which call `timetable/` and `parser/` for schedule data.
- Parser results are cached and normalized, then formatted via `src/formatter/` for text or image output.
- Output is delivered through the respective platform adapter or API response.

## Platform Entrypoints
- Telegram: `src/services/bots/tg/index.ts`
- VK: `src/services/bots/vk/index.ts`
- Viber: `src/services/bots/viber/index.ts`
- Alice: `src/services/alice/index.ts`
- VK App: `src/services/vk_app/index.ts`
- HTTP API: `src/services/api/index.ts`

## Where to Add New Code
- New bot command: add file under `src/services/bots/commands/<area>/` and register it in the relevant command index.
- New API method: add handler in `src/services/api/methods/` and export from `methods/_default.ts`.
- New parser type: extend `src/services/parser/types/` and update `parser/index.ts`.
- New formatter: implement in `src/formatter/` and wire in `src/formatter/index.ts`.

## Build, Test, and Development Commands
- Recommended Node.js runtime: LTS 22.x for local/dev/prod consistency.
- Package manager: `pnpm` (pinned via the `packageManager` field in `package.json`). Enable it with Corepack: `corepack enable` (ships with Node.js 16.10+).
- `pnpm install` installs all dependencies, including dev tools like `tsx`. The repository lockfile is `pnpm-lock.yaml`.
- `.npmrc` sets `shamefully-hoist=true` so transitive modules referenced directly in source (`google-auth-library`, `middleware-io`, etc.) resolve without extra setup.
- On startup, `Timetable` self-heals `timetable_archive` from `cache/rasp/*.json`: if the max `day` in the file cache is greater than the max `day` in the archive table, the cache is flushed into the DB via the same idempotent `appendDays` path (`updateOnDuplicate: ['data']`). This prevents stale archive rows from hiding valid schedule data after DB swaps or when the parser skips a fresh run because the file cache already looks up-to-date.
- If tests fail with `Cannot find module '../build/Release/canvas.node'`, run `pnpm rebuild canvas`.
- `pnpm start` runs the bot via `tsx src/bootstrap.ts` (runtime entry: `src/index.ts`).
- `src/bootstrap.ts` patches Node 25 `SlowBuffer` compatibility for `googleapis` transitive deps (`jws/jwa`).
- `pnpm run ts-check` runs `tsc --noEmit` for type checking only.
- Test runner: `vitest`. Configuration lives in `vitest.config.ts`.
- `pnpm test` / `pnpm run test:all` runs the full vitest suite once.
- `pnpm run test:watch` runs vitest in watch mode for TDD.
- `pnpm run test:coverage` runs the suite with V8 coverage; results go to `coverage/` (gitignored).
- `pnpm run test:logging` runs logging tests (`tests/logging/*.test.ts`).
- `pnpm run test:parser-v2` runs parser v2 tests (`tests/parser/*.test.ts`).
- `pnpm run test:bot-flows` runs Telegram command-regexp flow checks (`tests/bots/*.test.ts`).
- Source of truth for scripts: `package.json`.
- `pnpm exec tsx tests/inputTest.ts` runs the legacy interactive input script (not part of the vitest suite).
- `pnpm exec tsx scripts/findGroupBySameDays.ts` runs the utility script.
- `pnpm exec vitest run tests/parser/v2-fixtures.test.ts` re-runs parser v2 fixture checks after table-selection changes.

## Verification Checklist
- `pnpm run ts-check` for type safety after parser or command changes.
- `pnpm run test:logging` after logger/correlation/redaction changes.
- `pnpm start` for a smoke run of parser/bot behavior (manual check).
- `pnpm exec vitest run tests/parser` after parser changes.
- `pnpm run test:parser-v2` after parser v2 parser/diff/validate changes.
- `pnpm run test:bot-flows` after command regexp, menu text, or command routing changes.
- `pnpm run test:all` before release or before merge of large refactors.
- If you add tests, list the exact `pnpm exec vitest run <path>` command in the PR description.
- `GET /api/parser-health` (API key required) for parser status and metrics.
- API rate limiting: `config.api.rateLimit` (defaults: 120 req/min/ip). Keyed strictly by IP to prevent token-rotation bypass. Responses use IETF `RateLimit*` headers (`draft-7`). Set `trustProxy: true` behind a reverse proxy so the real client IP is used.
- `GET /api/health` (no auth) returns `{ ok, uptime, services, parserOk }` — safe for container healthchecks and load balancer probes.
- `GET /api/metrics` (no auth) returns Prometheus text-format metrics: `bot_up`, `bot_uptime_seconds`, `bot_services_enabled`, `bot_service_enabled{service=...}`, `bot_parser_ok`, `bot_parser_last_update_timestamp_seconds`, `bot_parser_staleness_seconds`, `bot_process_memory_bytes{area=...}`.

## Current Features Snapshot
- Telegram runtime is based on `grammY` long polling (`src/services/bots/tg/index.ts`).
- Telegram runtime always uses `grammY` long polling (`src/services/bots/tg/index.ts`).
- Telegram command registration is set via `setMyCommands` for default and admin scopes.
- Telegram command metadata type is defined in `src/services/bots/types/telegram.ts`.
- Telegram main menu includes `Google Calendar` button when `google_calendar` service is enabled.
- Telegram main menu includes `ICS` button when `calendar.ics.enabled` is true.
- ICS export command supports `/ics`, `ics`, and `📅 ICS` (Telegram).
- Formatter list includes compact formatter (`Компактный`) via `/formatter`.
- Diff settings menu exists under settings: `📊 Сравнение` with base and advanced submenus.
- Key user commands support button text and `/command` forms where applicable.
- If next-week schedules are removed after being published, the bot notifies users that the week was withdrawn.

## Settings Map
- Display settings menu: `src/services/bots/commands/settings/view/*`.
- Diff settings menu: `src/services/bots/commands/settings/diff/*`.
- Settings keyboard builders: `src/services/bots/keyboard/keyboard.ts`.
- Chat settings storage model: `src/services/bots/chat/Chat.ts`.
- Schema patching for new chat fields: `ensureBotChatSchema` in `src/services/bots/chat/Chat.ts`.
- Diff-related chat fields: `diffEnabled`, `diffAutoInWeek`, `diffAutoInUpdates`, `diffShowBeforeAfter`, `diffMaxLines`.
- Existing display fields: `showHints`, `showParserTime`, `hidePastDays`.

## Parser v2
- Enable in `config.ts` via `parser.v2.enabled`.
- Keep `parser.v2.fallbackToV1` true for safe rollout.
- Use `parser.v2.weekPolicy = 'preferCurrent'` to avoid switching to a future week.
- Use `parser.v2.sundayHoldCurrent = true` to avoid switching to a future week on Sundays.
- Use `parser.v2.hashMode = 'tables'` to reduce noise from layout changes.
- Use `parser.v2.quarantine` to block suspicious updates (too few lessons).
- Raw HTML options: `parser.v2.rawHtml.enabled`, `dir`, `maxDays`, `storeDaily`, `replayPath`, `diffMaxLines`.
- Metrics options: `parser.v2.metrics.enabled`, `dir`.

## Coding Style & Naming Conventions
- TypeScript is strict; keep `strict` assumptions (null checks, no implicit any).
- Indentation is 4 spaces as in `tsconfig.json`.
- Naming: `camelCase` vars/functions, `PascalCase` classes/types, `SCREAMING_SNAKE_CASE` constants.
- Do not add code comments.
- Keep service boundaries: cross-service access goes through service APIs, not internal files.
- Russian text in source files must be stored as Unicode (UTF-8) characters, not escaped bytes.
- Reasons:
- Readability: reviewers can read literals in code, not mojibake.
- Safety: prevents garbled output in logs/bots on different OS/terminal encodings.
- Portability: reduces cross-platform encoding surprises in CI and deployments.
- Where it applies:
- User-facing strings in commands, formatter, keyboards, and errors.
- Static text in `defines.ts`, command descriptions, and help prompts.
- Test fixtures and expected outputs.
- Where it may not apply:
- Binary payloads or intentionally encoded data (e.g., base64, hashes).
- External data stored as-is (e.g., parser HTML snapshots) unless you are editing human text.
- Examples:
- Good (Unicode in code):
```ts
context.send('Выберите группу в настройках (/setup)');
```
- Bad (escaped bytes / mojibake):
```ts
context.send('\\xD0\\x92\\xD1\\x8B...');
```
- Scenario: you see mojibake in output
- Fix: replace the literal with proper UTF-8 characters (retype the string).
- Scenario: adding new command text
- Rule: always type Russian text as normal Unicode characters in the source.
- Scenario: copying from logs or terminal
- Rule: verify the string displays correctly before committing.
- Review checklist for Russian strings:
- The string displays correctly in the editor (no mojibake).
- The string renders correctly in bot output/logs when tested.
- No accidental escape sequences or byte artifacts in source.

## Command/Input UX Rules
- If a command is reachable from a text button, its regexp should support that button text.
- Prefer supporting both `/command` and plain text input for user-facing commands.
- For Telegram-only behavior, declare it explicitly in command class (`services` or `requireServices`) and in docs.
- When adding a new main-menu button, add or verify matching command routing in the same PR.

## Testing Guidelines
- Test runner: `vitest` (see `vitest.config.ts`).
- Name tests `*.test.ts` and group them under `tests/<domain>/` (e.g. `tests/logging/`, `tests/parser/`, `tests/bots/`). Co-located `src/**/*.test.ts` files are also picked up.
- Keep tests deterministic and offline — do not require network/API calls.
- Prefer pure-function tests for parser/logging utilities and regexp/flow tests for command routing.
- If you add a new test file, list the exact `pnpm exec vitest run <path>` (or `pnpm test -- <filter>`) command in the PR description.

## How To Read Test Results
- Use `pnpm run test:all` (alias for `vitest run`) as the main gate.
- Expected output ends with a `Test Files  N passed (N)` / `Tests  M passed (M)` summary and exit code 0.
- Any failing assertion fails the whole run; vitest prints the diff and the source location.
- After `pnpm run test:all`, always run `pnpm run ts-check`.

## Coverage In This Project
- V8 coverage via `@vitest/coverage-v8`. Run locally with `pnpm run test:coverage`.
- CI uploads `coverage/` as an artifact (retention 14 days).
- Thresholds are enforced in `vitest.config.ts` and the `coverage` CI job fails on regression. Current floors: statements 18%, branches 13%, functions 22%, lines 18%. Raise these numbers in the same PR that adds coverage so the floor ratchets up.
- PR template for this repository:
- `Commands run:` `pnpm run test:all`, `pnpm run ts-check`
- `Result:` pass/fail + first failing test file (if any)
- `Scenario coverage:` changed module -> validating vitest test files

## Test Runner Conventions
- The runner is `vitest` — do not add a second runner.
- Prefer `describe` / `it` with explicit `expect(...).toBe(...)` / `toEqual(...)` assertions; avoid `assert.*` in new tests.
- Use `it.skip` / `describe.skip` with a short reason instead of commenting tests out.
- For async-context / ALS code paths, use real async/await in tests rather than fake timers.
- Keep test setup light-weight; avoid importing the whole `App` or starting services in unit tests.

## Test Execution Order
- Fast local check: `pnpm run test:logging`.
- Parser-focused check: `pnpm run test:parser-v2`.
- Bot routing check: `pnpm run test:bot-flows`.
- Full suite: `pnpm run test:all`.
- Type safety gate: `pnpm run ts-check`.

## Detailed Testing Workflow
- For logging changes:
- run `pnpm run test:logging`
- verify both context propagation and redaction assertions are green
- For parser v2 changes:
- run `pnpm run test:parser-v2`
- ensure fixture, validation and diff scripts all pass
- For Telegram command/menu/regexp changes:
- run `pnpm run test:bot-flows`
- verify both button-text and slash-command inputs are accepted where expected
- For large refactors touching multiple domains:
- run `pnpm run test:all`
- run `pnpm run ts-check`
- run `pnpm start` and confirm startup does not fail early
- Failure policy:
- if any script fails in `test:all`, stop release/merge
- fix failing domain first, rerun targeted test script, then rerun `test:all`
- if `ts-check` fails, do not merge even if all tests are green

## Test Coverage Targets
- Logging: verify ALS context propagation (`traceId/requestId/updateId`) and redaction behavior.
- Parser v2: verify fixture parsing, validation edge-cases, and diff output.
- Bot flows: verify command regex compatibility for button text and slash commands (Telegram-first).

## Documentation
- Keep user-facing docs focused on usage; move internals to developer docs.
- Add examples for new public APIs when helpful.
- Update changelogs for user-visible changes.
- Ensure links are valid and use Markdown links instead of raw paths.
- Avoid jargon without explanation in user docs.
- Google Calendar setup guide (Russian): [docs/google-calendar.md](docs/google-calendar.md).
- Keep AGENTS/README/docs synchronized for user-visible feature additions.

## Docs Sync Checklist
- If behavior is user-visible, update `AGENTS.md` and `README.md` in the same PR.
- If feature has setup flow, add or update a focused doc under `docs/` and link it from AGENTS/README.
- If commands or settings changed, update command/settings sections and examples.

## Commit & Pull Request Guidelines
- Follow existing commit style: short, imperative summaries in English, <= 72 chars.
- Prefer a `type:` prefix (`feat`, `fix`, `docs`, `refactor`, `chore`) when it fits the change.
- Commit messages must conform to Conventional Commits. Emojis are optional; if used, follow Gitmoji.
- Official references (latest at time of update, copy/paste):
```
https://www.conventionalcommits.org/en/v1.0.0/
https://gitmoji.dev/specification
```
- Conventional Commits format: `<type>[optional scope][!]: <description>`
- Optional body and footer start after a blank line.
- Use `!` or `BREAKING CHANGE:` for breaking changes (see rules below).
- Core rules:
- `type` is required and must be lowercase (e.g., `feat`, `fix`, `docs`, `refactor`, `chore`, `test`, `build`, `ci`).
- `scope` is optional and should be a short area (e.g., `parser`, `bots`, `api`, `formatter`).
- `scope` is written in parentheses, e.g. `feat(parser): ...`.
- `description` is required, imperative, no trailing period.
- Breaking changes require both:
- `!` after type/scope, and
- A `BREAKING CHANGE:` footer describing the impact.
- Examples (based on recent history, translated to English):
- `fix: remove prefix for own notifications`
- `feat: add parser v2 and tests`
- `fix: avoid switching to next week on empty data`
- `feat: add subscriptions and start schedule`
- `docs: extend agents guidance`
- `fix: update parser for new site layout`
- `feat: harden parser v2 and add health check`
- `chore: bump ws to 8.17.1`
- `refactor: rewrite date/time utils`
- `fix: show week timetable button on alerts`
- Breaking change example:
- `feat(parser)!: drop legacy v1 cache format`
- `BREAKING CHANGE: v1 cache files are no longer read; reparse is required.`
- Optional Gitmoji format (if team decides to use emojis): `<emoji> <type>(scope)?: <description>`
- Gitmoji example: `:sparkles: feat(parser): add v2 health check`
- PRs should include a clear description, related issues, and the commands you ran (e.g., `pnpm run ts-check`).
- For behavior or asset changes, include before/after notes.
- Always check `git status` before committing.
- Stage files explicitly (e.g., `git add path/to/file`); do not use `git add .`.

## Configuration & Security
- Copy `config.example.ts` to `config.ts` for local setup.
- Keep secrets out of git and avoid committing local DB files (e.g., `sqlite3.db`).

## Bot Behavior Notes
- Core flow: `src/index.ts` bootstraps; `src/app.ts` registers services and starts them after DB sync.
- Schedule parsing relies on site HTML; keep selectors tolerant to layout changes and add fallbacks.
- Parser cache lives under `./cache/rasp/` and emits update events; avoid clearing keys unless requested.
- Bots: commands live in `src/services/bots/commands/`, callbacks in `src/services/bots/callbacks/`, keyboards in `src/services/bots/keyboard/`.
- Telegram adapter uses `grammY` contexts and `ctx.api.*` calls in `src/services/bots/tg/*`.
- Timetable formatting lives in `src/formatter/`; domain objects live in `src/services/timetable/`.

