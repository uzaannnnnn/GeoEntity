# Agent Operating Guide

This guide applies to coding agents working in the GeoEntity repository. The human project owner controls scope and phase transitions. This document describes current repository practice; it does not authorize work by itself.

## Project architecture

- **Frontend:** React 19, TypeScript, Vite, Vitest, Leaflet, and React Leaflet. UI and map interaction live under `frontend/src/` and call the typed API client under `frontend/src/api/`.
- **Backend:** Go 1.27.3 module `geoentity/backend`, standard-library `net/http`, handlers and domain packages under `backend/internal/`, and command-line tools under `backend/cmd/`.
- **Persistence:** SQLite through `database/sql` and `modernc.org/sqlite`. Numbered embedded SQL migrations under `backend/internal/repository/migrations/` are applied and checksum-verified by the repository migration runner.
- **Runtime:** one API process per SQLite database. Vite's `/api` proxy is development-only. Production hosting and reverse proxy configuration are not supplied.

For current behavior, inspect executable source and tests. Use `README.md` for developer setup, `V2_ROADMAP.md` for milestone status, and phase reports for historical evidence. Plans and contracts can describe proposals; do not present a proposal as shipped behavior.

## Repository boundaries

- `backend/cmd/api/`: API process setup and runtime configuration.
- `backend/cmd/geoentity/`: administrator/device provisioning and backup/restore CLI.
- `backend/internal/auth/`: administrator authentication, session/CSRF middleware, and audit helpers.
- `backend/internal/handler/`: HTTP routing, DTO decoding, validation, and response codes.
- `backend/internal/`: entity, geofence, monitoring, backup, and maintenance logic.
- `backend/internal/repository/`: SQLite data access and migrations.
- `frontend/src/api/`: typed API client; `components/`: UI; `utils/`: pure helpers and validation.

Keep UI state and Leaflet-specific objects in the frontend. Keep SQL in repositories, domain rules in domain packages, and HTTP concerns in handlers. Preserve existing API contracts unless the authorized task explicitly changes them.

## Development conventions

- Inspect `git status --short --branch`, current `HEAD`, and relevant diffs before editing. Preserve all existing staged, unstaged, and untracked work.
- Make the smallest change that satisfies the authorized task. Do not add dependencies, features, schema changes, configuration, or public API surface without explicit scope.
- Backend request parsing must remain bounded and strict. Validate at the backend even when the frontend validates. Use parameterized SQL and avoid leaking internal errors, credentials, or hashes.
- Frontend changes must preserve keyboard use, focus visibility, responsive layouts, and existing component behavior. Do not claim responsive/browser QA without actual browser evidence and screenshots where requested.
- For UI changes, inspect component structure, CSS cascade, breakpoints, and pointer behavior before editing. Keep overlays and map controls functional at narrow widths.
- Update living docs when current commands, routes, configuration, or limitations change. Keep historical test results and chronology unchanged; use a dated correction/addendum when needed.

## Authentication and security constraints

- The app has one CLI-provisioned administrator. Passphrases are entered interactively and must not be placed in shell history, logs, source, tests, screenshots, or browser storage.
- Browser sessions are opaque, stored server-side by token hash, and use HttpOnly cookies. Secure cookies are the default. `GEOENTITY_DEV_COOKIE_INSECURE=true` is only for a loopback development bind.
- Unsafe administrator browser requests require a valid session, configured exact Origin, and `X-CSRF-Token`. Do not weaken or bypass this middleware.
- Only the exact `/api/device/v1/observations` path uses device bearer credentials; all other protected API routes remain administrator-authenticated. Raw device credentials are one-time display values; never persist them in frontend state longer than required or expose them in API responses/logs.
- Configure trusted proxy CIDRs narrowly. Do not trust forwarded headers from arbitrary peers. Do not make claims of production deployment, TLS, multi-host rate limiting, or recovery testing without evidence.

## Database and migration safety

- Never open, inspect, query, modify, migrate, replace, or back up a protected persistent database. Use temporary isolated databases only when authorized and necessary.
- Do not edit an already-applied migration. Add a new numbered migration only when schema work is explicitly authorized. Include fresh and upgrade-path tests, rollback/integrity coverage, and update schema/backup compatibility where required.
- Do not use the database's default path for tests or experiments. Check test setup and process environment before running commands that can open a database.
- SQLite is single-writer; the supported application model is one API process per database. Rate limits are process-local.

## Verification

Use the scripts and commands documented in `README.md`:

- Backend, from `backend/`: `go test ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`.
- Frontend, from `frontend/`: `npm test`, `npm run build`.
- Documentation/code hygiene: `git diff --check`.

Run only checks relevant to the authorized task, in the authorized worktree. Do not install packages unless explicitly authorized. Report exact commands and results. A cached test result is evidence of command completion but should be described as cached where known. Unit tests, source inspection, and builds are not browser QA. Never claim a check passed unless it was actually run.

## Git and authorization rules

- Never reset, clean, restore, stash, rebase, filter history, or discard existing changes unless the owner explicitly authorizes that exact operation.
- Do not stage, commit, push, tag, release, deploy, or change Git identity/configuration without separate explicit authorization.
- Work only in the repository/worktree specified by the current task. Do not read or change another repository as a convenience.
- Preserve historical commits, reports, failed QA findings, and prior verification evidence. Do not rewrite them to make a past result appear better.
- Stop before scope expansion and report concrete blockers or unverified assumptions. Human approval is required for new feature scope, schema/API changes outside the prompt, publication, and deployment.

## Documentation maintenance

README.md is the current developer entry point. Keep its prerequisites, setup, environment variables, API index, security statements, and limitations tied to source and package scripts. `AGENT.md` is the operating guide. `V2_ROADMAP.md` describes phase status. Plans/contracts describe their phase's proposal or frozen contract. Handoffs and QA reports are historical evidence unless a clearly dated addendum is appended.

Inventory Markdown files before a documentation overhaul. Check local links, headings/code fences, environment variable names, API routes, shell commands, and stale local paths. Mark unverifiable statements as unverified instead of guessing. Do not invent license, contribution, deployment, or browser support policies.
