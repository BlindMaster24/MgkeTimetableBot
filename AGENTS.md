# Repository Guidelines

## Project Structure & Module Organization
- Entrypoints: `src/index.ts` bootstraps the app; `src/app.ts` registers services and starts them (DB sync, then `service.run()`).
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
- `npm install` or `yarn install` installs dependencies.
- `npm start` or `yarn start` runs the bot via `ts-node .` (entry: `src/index.ts`).
- `npm run ts-check` or `yarn ts-check` runs `tsc --noEmit` for type checking only.
- `ts-node tests/inputTest.ts` runs the existing test script.
- `ts-node scripts/findGroupBySameDays.ts` runs the utility script.
- `ts-node tests/parserV2Test.ts` runs parser v2 fixture checks.

## Verification Checklist
- `npm run ts-check` or `yarn ts-check` for type safety after parser or command changes.
- `npm start` or `yarn start` for a smoke run of parser/bot behavior (manual check).
- `ts-node tests/parserV2Test.ts` after parser changes.
- If you add tests, list the exact `ts-node ...` command in the PR description.

## Parser v2
- Enable in `config.ts` via `parser.v2.enabled`.
- Keep `parser.v2.fallbackToV1` true for safe rollout.
- Use `parser.v2.weekPolicy = 'preferCurrent'` to avoid switching to a future week.

## Coding Style & Naming Conventions
- TypeScript is strict; keep `strict` assumptions (null checks, no implicit any).
- Indentation is 4 spaces as in `tsconfig.json`.
- Naming: `camelCase` vars/functions, `PascalCase` classes/types, `SCREAMING_SNAKE_CASE` constants.
- Do not add code comments.
- Keep service boundaries: cross-service access goes through service APIs, not internal files.

## Testing Guidelines
- No test runner is configured; tests are scripts.
- Name tests `*Test.ts` under `tests/` and keep them deterministic.
- If you add a new test, note the command in the PR description.

## Commit & Pull Request Guidelines
- Follow existing commit style: short, imperative summaries (often Russian), <= 72 chars.
- Examples:
  - feat: improve pluralization for admin actions
  - feat: localize root channel label
  - refactor: unify shutdown flow and state storage
  - fix: handle replies from message bot
  - docs: avoid code comments unless requested
  - chore: prepare 0.4.2 release
- PRs should include a clear description, related issues, and the commands you ran (e.g., `npm run ts-check`).
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
- Timetable formatting lives in `src/formatter/`; domain objects live in `src/services/timetable/`.
