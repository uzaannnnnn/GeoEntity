# GeoEntity V2 — Phase Handoff

## V2.1 final status

V2.1A–V2.1D implementation, integration, and independent QA are complete. V2.1E documentation and final regression are complete; final Git closure is authorized and is being completed with one commit. The user-provided V2.1D QA report verdict is **PASS**, with no P0 or P1 issues and all V2.1 regression tests passing. V2.2 has not started.

## Phase results

- **V2.1A — Safe database evolution:** ordered embedded SQLite migrations, `schema_migrations` ledger (version, name, SHA-256 checksum, UTC `applied_at`), fail-closed legacy V1 schema recognition, transactional application, rollback and repeat-startup tests, and V1 API compatibility coverage.
- **V2.1B — Map provider compliance:** official OSM tile URL `https://tile.openstreetmap.org/{z}/{x}/{y}.png`; visible attribution; environment-configurable URL, attribution, and max zoom with defaults; no legacy `{s}` URL, tile prefetch, custom cache override, or restrictive referrer policy found in source.
- **V2.1C — Full integration:** Go API and Vite `/api` proxy exercised against an isolated temporary SQLite database. Health/list/create/detail/update with changed coordinates/validation error/delete passed; a record persisted after backend restart. V1 API contract remained compatible.
- **V2.1D — Independent QA:** user-provided report verdict **PASS**; no P0 issues, no P1 issues, and all V2.1 regression tests passed.
- **V2.1E — Sprint closure:** roadmap and handoff finalized, regression rerun, Git identity configured locally as explicitly instructed, and only intended V2.1 files are being prepared for the authorized commit.

## Database preservation evidence

Before V1 baseline registration, a consistent SQLite backup was created outside Git and independently opened read-only:

`[LOCAL_PATH_REDACTED] Support/GeoEntity/backups/geoentity-20261102T135501Z.db`

The backup schema and both existing records matched the source. SHA-256: `c847028ba472bfd29b92b319ec4afadabaf5a6a709d5648d170f1f245c886acf`. After migration, exact entity row values still matched the backup, and the ledger recorded migration 001. During V2.1C the persistent database was inspected read-only; it still contained two records and the V1 ledger. The backup and live DB are excluded from Git and were not included in staging. Restore only while the API is stopped: copy the backup over `backend/data/geoentity.db`, then restart.

## Verification

Backend commands run from `backend/`:

- `go test -count=1 ./...` — passed.
- `go test -race -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/api` — passed.

Frontend commands run from `frontend/`:

- `npm test` — 6 test files, 28 tests passed.
- `npm run build` — TypeScript check and Vite production build passed (92 modules transformed).

V2.1C HTTP integration through Vite’s `/api` proxy passed for health, empty list, create, detail, coordinate update, invalid latitude response (`400 VALIDATION_ERROR`), restart persistence, and deletion (`204`, empty list afterward). Migration tests use isolated databases and cover empty initialization, two-row legacy V1, row preservation, repeated startup/reopen, checksum mismatch, unexpected schema/index/constraint, rollback, and ledger consistency. No user DB was used as a destructive test fixture.

`git diff --check` passed. Package manifests and lockfiles did not gain dependencies. Root `.gitignore` excludes `/backend/data/`, `*.db`, `*.db-shm`, and `*.db-wal`; frontend `.gitignore` excludes `node_modules` and `dist`. Build artifacts and the temporary integration DB were removed after verification. No browser-driven visual/E2E test was run because the project has no browser test harness; map configuration was verified by source inspection, unit tests, and production build.

## Intended V2.1 files

- `backend/internal/repository/sqlite.go`
- `backend/internal/repository/migrations_test.go`
- `backend/internal/handler/api_test.go`
- `frontend/src/utils/mapConfig.ts`
- `frontend/src/utils/mapConfig.test.ts`
- `frontend/src/components/MapView.tsx`
- `frontend/src/vite-env.d.ts`
- `frontend/.env.example`
- `V2_ROADMAP.md`
- `V2_ARCHITECTURE.md`
- `V2_CONTRACT.md`
- `V2_HANDOFF.md`

No SQLite database, backup, `node_modules`, build output, local `.env`, or original take-home repository file is included in the staged deliverables. The intended author identity is configured locally for this repository only.

## Remaining limitations and next action

- Browser visual tile rendering and marker disappearance were not independently checked in a browser.
- OSM standard tiles remain best-effort without an SLA; public/high-volume deployments need a provider whose terms and capacity fit the deployment.
- The API remains unauthenticated and should stay local/trusted-network only.
- V2.1 closure commit message: `feat: establish GeoEntity V2.1 foundation`.
- **Next product phase: V2.2 Map Intelligence.** Start only after V2.1 Git closure and explicit human authorization. Do not begin it automatically.
