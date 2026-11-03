# GeoEntity V2 — Product Roadmap

**Status:** V2.1–V2.5 are completed and locally committed. V2.5 is commit `43ce1a7d51950a6cc0ec79692cc3b779b252340b`. V2.6A–D are complete; V2.6 local QA recorded 0 P0, 0 P1, 3 P2, and 1 P3. Production deployment readiness remains subject to operational verification; see `V2.6_FINAL_GIT_REPORT.md`. V2.7A–D are complete from baseline `9d7871e50f6d59befcffe675054e2d5fb373ef5c` (author `Muhamad Fauzan <vanz66hu@gmail.com>`). Original V2.7C QA returned FAIL; findings were remediated and the responsive discrepancy passed independent re-verification. The original FAIL and prior conflicting screenshot evidence remain historical. V2.7D was finalized in one local commit. V2.8A integration/QA is complete with browser verification explicitly excluded because browser automation was unavailable; V2.8B has not started.

## Product direction

Evolve the current single-user GeoEntity CRUD map into a spatial asset and IoT operations platform while retaining the React/TypeScript, Go, Leaflet, and SQLite foundation. Keep the existing API and entity data usable through additive changes. Do not introduce services, infrastructure, or dependencies until a roadmap sprint requires them.

## Workspace and current system inventory

The active V2 workspace is `./`, a separate Git repository from the original take-home project at `[LOCAL_PATH_REDACTED]`. V2.5A verified branch `master`, HEAD `8486b20207b68bcf5bb115b6aa414630f4094442`, and a clean starting worktree. The original take-home directory was not modified.

The V2 copy has a local, Git-ignored `backend/data/geoentity.db` with two entity rows, a V1 baseline in `schema_migrations`, and `PRAGMA user_version = 0`. A timestamped backup was independently verified before baseline registration; preserve both data and backup. Neither belongs in Git.

| Area | Implemented now | V2 gap |
|---|---|---|
| Map | Leaflet/React Leaflet, configurable official OSM raster URL, attribution, zoom, custom entity markers, marker selection, auto-fit on initial/manual refresh, draft location click/drag; V2.2 adds persisted-marker clustering, Locate Me, Fit All, legend, coordinate inspector/copy, and accessible controls | No basemap switcher; production rendering performance is not benchmarked |
| Entities | V1 CRUD/list; V2.3 custom attributes, tags/categories and append-only activity; V2.4B server-side search/cursor pagination; V2.4E frontend CSV/GeoJSON operations with backend preview/commit and export | Legacy V1 list remains unpaginated for compatibility |
| Geospatial | V2.5B circle/simple Polygon definitions and geometry; V2.5C derived membership and coordinate-transition history; V2.5D map overlays, management, membership UI, filtered event timeline | Geometry remains deliberately bounded; no telemetry/live tracking |
| IoT | Migration 007 device registry/hashed credentials, bounded authenticated single-observation ingestion, idempotent append-only sensor history, monitoring/history APIs, and focused dashboard | Single-process local SQLite; indefinite retention; no thresholds, simulator, external workload target, or multi-host limiter |
| Backend | Go `net/http`, SQLite, existing entity/geofence/security/backup APIs plus exact device ingest and admin monitoring/history routes; device lifecycle CLI | V1 full-list endpoint remains for compatibility; device lifecycle is operator CLI based |
| Database | Migrations 001–007; devices, credentials, immutable sensor-unit registry and append-only observations | No V2.7 membership/event tables; indefinite retention grows DB/backups; rollback after telemetry requires roll-forward |
| Quality | V2.7B automated tests/build, original V2.7C independent QA, targeted remediations, responsive re-verification, and V2.7D final verification are recorded in their reports | Original V2.7C FAIL remains historical. Responsive re-verification passed at 390×844, 768×1024, and 1440×900 with dashboard checks. Larger scale and soak remain incomplete |

### Current implementation quality

The current code is a small, understandable monolith with clear HTTP handler, entity validation, and SQLite repository boundaries. API error envelopes are consistent, invalid entity input is rejected independently by the backend, and frontend mutations apply their successful response before reconciling the list. This is a sound base for incremental evolution.

The V2.1 runner now handles persistence evolution: numbered migrations are ordered, checksummed, and recorded transactionally. V1 recognition checks the legacy table, constraints, columns, and index. `PRAGMA user_version` intentionally remains zero; `schema_migrations` is authoritative.

The V1 entity list endpoint continues to load all rows for backward compatibility. The V2.4 frontend uses the bounded keyset-paginated query endpoint; separate map and export queries are independent of that page. The Go repository limits the SQL pool to one open connection, which serializes access and suits this MVP but will constrain concurrent ingestion and reads.

## Roadmap levels and acceptance criteria

### Level 1 — Map Intelligence

1. **Marker clustering**
   - Cluster only saved entity markers; preserve selected-entity details and marker selection after zooming into a cluster.
   - Do not cluster a draft location marker.
   - Acceptance: dense and sparse marker sets remain selectable; no duplicate markers appear after CRUD refresh; the map remains responsive with a documented representative dataset.
2. **Locate Me**
   - Use the browser Geolocation API only after the user action; show permission-denied, unavailable, and timeout states; do not persist the user's device location as an entity.
   - Acceptance: permission is requested on demand, the view moves to the returned coordinate, and no location is transmitted to the backend without a product requirement.
3. **Fit All Entities**
   - Fit valid entity points with sensible padding and a useful maximum zoom; handle zero and one entity explicitly.
   - Acceptance: one marker is centered, multiple markers fit, and invalid coordinates never reach Leaflet.
4. **Map controls, legend, and coordinate inspector**
   - Expose map operations with accessible labels; show type/status encoding; allow coordinate inspection/copy without changing records.
   - Acceptance: the meaning of marker appearance is visible and coordinates are shown in a documented precision/order.
5. **Basemap selection**
   - Select a provider only after reviewing its current URL, usage limits, caching, attribution, availability, privacy, and commercial terms. Keep OSM attribution when OSM-derived data is displayed.
   - Acceptance: configured providers are switchable without a code release, attribution stays visible, and no unsupported prefetch/offline behavior is introduced.

### Level 2 — Advanced Entity Management

1. **Custom attributes, tags, and categories — completed in V2.3**
   - Define whether attributes are per entity type or per entity; define allowed JSON types, size limits, validation, and filter semantics before schema work.
   - Keep the existing `type` field as-is; categories/tags are separate concepts.
   - Acceptance: old entity requests still succeed; new attributes are validated on both client and server; tag/category relations do not duplicate.
2. **Advanced search and filtering — implemented in V2.4B/C**
   - Add server-side query parameters with deterministic ordering and cursor/limit bounds. Preserve the existing `{data: [...]}` response shape and add optional top-level metadata only.
   - Acceptance: combined filters, empty result, malformed filter, page boundaries, and stable cursor behavior are covered.
3. **CSV and GeoJSON import/export — completed in V2.4D–V2.4H**
   - Support preview/validation before writing. Validate every row/feature and all references before opening the write transaction. Initial commit mode should be all-or-nothing with a documented duplicate-ID policy and file/row limits.
   - Acceptance: valid import, mixed-invalid preview, invalid file rejection, duplicate handling, transaction rollback, export round-trip, and data-limit behavior are tested.
4. **Activity timeline — completed in V2.3; imported creation events supported by V2.4D**
   - Append events for entity create/update/delete and completed imports. Events are append-only; corrections are new events, never edits to history.
   - Acceptance: a failed entity write creates no success event; a successful write and its event commit atomically; timeline paging is stable.

### Level 3 — Geospatial Intelligence

1. **Circle and polygon geofences**
   - Define GeoJSON coordinate order as `[longitude, latitude]`; support a deliberately bounded geometry subset first (circle and Polygon; decide holes/MultiPolygon separately). Document boundary inclusion and antimeridian behavior.
   - Acceptance: geometry validation and containment tests cover boundary, outside/inside, malformed ring, degenerate polygon, holes if enabled, antimeridian, and high-latitude cases.
2. **Membership and entry/exit detection**
   - Silently establish baselines on creation/import and geofence create/update/enable. On coordinate changes, compare persisted membership and atomically update entity, membership, activity and transition history. Disable marks retained membership inactive; delete removes current state but preserves snapshots.
   - Acceptance: retries are idempotent; concurrent updates cannot create contradictory transitions; a failed transaction leaves no partial membership/event writes. V2.5C backend is complete; V2.5D presents these records.
3. **Geofence event history, alert center, spatial visualization**
   - Provide filterable, paginated event history and alert acknowledgement only if its semantics are defined. Do not call an event real-time unless the source and delivery latency are measured.

### Level 4 — IoT Monitoring

1. **Telemetry records and ingestion API**
   - Define metric, unit, event time, receive time, source, idempotency key, and retention before designing the table/API. Protect ingestion before exposing it externally.
   - Acceptance: authentication/rate limits are active for public ingestion, duplicate submissions are idempotent, timestamps are normalized to UTC, invalid metrics do not persist, and bounded batch ingestion is tested.
2. **Sensor history, battery/signal monitoring, threshold alerts**
   - Keep telemetry append-only and separate from entity current-state attributes. Define retention/aggregation and alert hysteresis before implementation.
3. **Demo simulator**
   - Optional and explicitly labeled “Demo”; disabled by default in production and isolated from real ingestion.
   - Acceptance: generated data has a `source=demo` marker, can be reset/removed, and can never be mistaken for live device readings.

### Level 5 — Enterprise Evolution

- Authentication and role-based access control; production threat model, secrets handling, audit/retention controls, deployment, and monitoring.
- PostgreSQL/PostGIS evaluation against measured data volume, concurrent writes, spatial query latency, and operational needs. Migration is a separate project with backup/restore and parity tests, not a roadmap assumption.
- Advanced analytics only after operational questions, required aggregates, and data retention are defined.

**Dependency ordering note:** enterprise access control is listed in Level 5, but it is a release gate before telemetry or any other externally reachable write API. If those APIs are needed earlier, move the minimal authentication/API-protection sprint forward rather than exposing unauthenticated ingestion.

## Recommended sprint sequence

| Sprint | Scope | Depends on | Exit criteria |
|---|---|---|---|
| V2.0 — Planning baseline | This inventory, V2 architecture/contract/roadmap, preserve the existing DB and API | None | Human approves scope, data semantics, and next sprint; no feature code changed |
| V2.1 — Safe evolution foundation | **COMPLETED (V2.1A–V2.1E).** Versioned SQLite migrations, legacy DB backup/baseline, compatibility tests, OSM URL/config compliance, integration and independent QA | V2.0 approval | Existing two-row DB preserved; migration rerun safe; V1 API tests pass; OSM configuration and frontend/backend regression pass |
| V2.2 — Map Intelligence | **CLOSED (V2.2A–V2.2E).** Persisted-entity clustering, one-shot Locate Me, Fit All, legend, coordinate inspector/copy, accessible toolbar; no backend/API/schema changes | V2.1; existing map | Backend/frontend regressions and isolated HTTP CRUD verification pass; V2.2D QA verdict PASS with zero P0/P1 findings per user. Browser interaction verification is a documented limitation because no browser executable/harness was available; local API benchmarks do not claim browser performance. OSM attribution/provider compliance retained. |
| V2.3 — Entity model and activity | **COMPLETED.** Typed custom attributes, tags/categories, append-only mutation activity, additive migration 002 | V2.1 | V1 clients remain compatible; metadata validated; mutation+activity atomicity tested and committed at `7d508dab10bc44ea48cecbc8e47ba9ebad91db1c` |
| V2.4 — Data operations | **V2.4A–H COMPLETE.** Backend search/pagination and separate map queries, frontend query/map integration, bounded CSV/GeoJSON export and preview/commit import, browser-to-backend integration verification, independent QA, and favicon defect resolution | V2.3 | V1 routes remain unchanged; list pages do not truncate map/export data; imports validate before atomic transaction; preview token binds bytes/options; export/import, metadata round-trip, rollback, map refresh, favicon and regression gates verified |
| V2.5 — Geospatial intelligence | **V2.5A–E implementation and verification recorded.** A post-audit report alleged geofence deletion fails with event history; follow-up reviewed actual SQLite constraints and API/repository deletion path and did not reproduce it. See the remediation addendum in `V2.5_HANDOFF.md`; the historical V2.5E QA verdict is qualified by this evidence and does not authorize release. | V2.4 baseline; explicit authorization per phase; security gate before external exposure | Temporary-DB regression verifies membership cascade, retained/queryable immutable event history after entity/geofence deletion, and delete behavior with/without history; V1/V2.4 APIs remain compatible |
| V2.6 — Security & Reliability | V2.6A planning, V2.6B implementation, V2.6C QA passed in the tested local environment (0 P0, 0 P1, 3 P2, 1 P3), and V2.6D final Git review/local commit complete. Production deployment readiness remains subject to operational verification. | **V2.6A–D complete;** see `V2.6_QA_REPORT.md`, `V2.6_HANDOFF.md`, and `V2.6_FINAL_GIT_REPORT.md`. | Local session/CSRF/route tests, migration compatibility, immutable audit/history, verified backup and isolated restore drill, benchmark evidence, regression gates, and final Git review passed; production HTTPS proxy, off-device recovery, and host tuning remain operational checks |
| V2.7 — Essential IoT Monitoring | **V2.7A–D complete; original V2.7C QA FAIL remains historical. Targeted responsive remediation and independent responsive re-verification PASS are recorded.** See `V2.7C_QA_REPORT.md`, `V2.7B_REMEDIATION_REPORT.md`, `V2.7B_RESPONSIVE_REMEDIATION_REPORT.md`, `V2.7C_RETEST_REPORT.md`, and `V2.7_HANDOFF.md`. | V2.6 security foundation; all V2.7 phases complete | One local V2.7D commit; no push, tag, release, or deployment |
| V2.8 — Final Integration, QA & Release Closure | End-to-end verification, restore drill, security regression, measured performance review, deployment configuration, documentation and release acceptance | V2.6 and V2.7 implementation complete | All approved cross-version workflows pass; operator recovery is demonstrated; release criteria and known limits are signed off |

V2.4 phase ordering and authorization boundaries are specified in `V2.4_IMPLEMENTATION_CONTRACT.md`. Database foundation is a prerequisite for every backend feature sprint.

## Cross-cutting acceptance gates

- Every migration is additive where practical, transactional where supported, tested on a copy of the existing V1 database, and has a backup/restore plan.
- Existing endpoints, status codes, response envelopes, UUIDs, UTC timestamps, and type/status values remain compatible unless a documented major API decision is approved.
- Imports validate complete input before database writes and report row/feature number, field, stable error code, and message.
- Activity and telemetry history are append-only; retention/deletion policy is explicit.
- Geofence math is pure, deterministic, and covered by edge-case tests.
- No mock/demo telemetry is presented as live data.
- No new dependency is added without confirming maintenance, license, React/Leaflet/Go compatibility, bundle impact, and whether the standard library/current stack suffices.
- No public deployment is considered ready until authentication, authorization, rate limits, input/file limits, secrets, HTTPS, and logging/monitoring are reviewed.

## Main risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Legacy databases may have `user_version=0` and no ledger | A naïve migration can rebuild/drop live data or fail to upgrade | **Mitigated in V2.1:** verified external backup, exact V1 schema recognition, ledger baseline without row rewrites, ordered transactional migrations, checksum and rollback tests |
| Entity enums are duplicated in Go validation, SQLite CHECK constraints, and TypeScript unions | Adding status/type can fail writes or produce mismatched UI | Treat enums as contract; update database migration, API, frontend, and tests in one reviewed release; prefer category/tag tables for user-defined taxonomy |
| SQLite single connection and unbounded `GET /entities` | Import/telemetry writes block UI reads and list memory grows | Add pagination and workload benchmarks before high-volume features; keep SQLite initially and evaluate Postgres only from measurements |
| Strict unknown-field JSON decoder | Adding a front-end field before backend rollout returns 400 | Deploy/merge backend support before frontend sends fields; contract tests; additive rollout sequence |
| Public unauthenticated REST API | Data disclosure or destructive writes | Keep local/trusted-network only; require auth/rate limits before external exposure or telemetry ingestion |
| Geofence geometry edge cases | False entry/exit alerts, especially at boundaries/antimeridian/high latitudes | Decide coordinate and boundary semantics; use a reviewed geometry library candidate; test golden cases and transaction idempotency |
| Import partial writes or duplicate records | Corrupt or surprising asset records | Full preview/validation first, strict row/file limits, unique constraints, atomic all-or-nothing commit, explicit duplicate policy |
| Activity/telemetry volume and retention | Database growth and slower backups/queries | Separate append-only tables, indexes, retention/partition plan, export/backup tests; avoid placing time-series data in `entities` |
| OSM standard tile availability | Tiles can be throttled/blocked; map unavailable | **URL/config compliance completed in V2.1B:** uses `tile.openstreetmap.org`, retains attribution, and introduces no tile prefetch/cache override. Service remains best-effort with no SLA; use an appropriately licensed commercial or self-hosted provider for public/high-volume deployments. |

## V2.0 exit criteria

- [x] Correct V2 copy identified and original repository left untouched.
- [x] Current Git branch/tree, modules, API, database schema, migration, and local DB state inventoried.
- [x] Feature dependency order, migration/backward-compatibility strategy, risks, acceptance gates, and dependency candidates documented.
- [x] Human reviewed and explicitly authorized V2.1. No later sprint starts automatically.

## V2.1 closure

- [x] V2.1A–V2.1C implementation and integration checks passed.
- [x] V2.1D verdict PASS; no P0/P1 findings and all V2.1 regression tests passed, per user-provided report.
- [x] V2.1E handoff and roadmap updated; final V2.1 commit explicitly authorized; no V2.2 implementation started.

## V2.2 closure

- [x] V2.2A implementation contract and handoff prepared without source changes.
- [x] V2.2B implemented all six Map Intelligence features with `react-leaflet-cluster@4.1.3`; V1 API, backend source, and database schema were unchanged.
- [x] V2.2C backend/frontend regressions, production build, isolated Vite proxy HTTP CRUD, and restart persistence passed. Local API-only benchmark measurements were recorded separately from browser performance.
- [x] V2.2D verdict PASS with zero P0/P1 findings; source-level/architectural review passed all six Map Intelligence capabilities. User-authorized P2 observations: entity listing is unpaginated (pagination planned for V2.4), and SQLite uses one connection (concurrency evaluation deferred until telemetry workload requires it). The standalone QA report was unavailable in the workspace; this remains a documentation limitation, not evidence of audit failure.
- [x] V2.2E final backend/frontend regression passed and documentation updated. The authorized commit message is `feat: add GeoEntity V2.2 map intelligence`. Browser-driven cluster, geolocation, keyboard, and responsive checks remain unverified due to missing browser tooling.

## V2.3 closure

- [x] V2.3 implementation and integration completed: typed custom attributes, tags/categories and append-only activity events in an additive migration; V1 API behavior retained.
- [x] V2.3 final commit is `7d508dab10bc44ea48cecbc8e47ba9ebad91db1c` (`feat: complete GeoEntity V2.3 advanced entity management`).

## V2.4 Data Operations Progress

- [x] V2.4A: Planning, contracts, and handoff completed.
- [x] V2.4B: Backend search, keyset cursor pagination, migration 003 (FTS5 trigram), map viewport (`GET /api/entities/map`), and extent (`GET /api/entities/map/extent`) endpoints completed and verified with race detector and real HTTP daemon lifecycle.
- [x] V2.4C: Frontend server-side search, keyset pagination ("Load More"), independent map viewport fetching, server-extent "Fit All", off-page detail loading, 413 MAP_RESULT_LIMIT handling, 124 unit/integration tests passing, and real headless Chrome CDP end-to-end multi-viewport verification completed with 0 console errors.
- [x] V2.4D: Backend CSV/GeoJSON export and preview/commit import implemented with bounded parsing/export, V2.3 metadata validation and atomic entity/catalog/activity writes. Backend tests, race, vet, build, frontend regression and isolated real HTTP/restart checks passed. No schema/dependency changes or staging/commit.
- [x] V2.4E: Frontend CSV and RFC 7946 GeoJSON export and preview/commit import implemented. Includes Data Operations modal, active filter-scoped exports, spreadsheet-safe formula escaping, raw byte and token integrity verification, row/feature validation tables, CREATE-ONLY all-or-nothing confirmation, automated refresh, 134 passing Vitest unit tests, and real headless Chrome CDP end-to-end multi-viewport verification with 0 console errors.
- [x] V2.4F: Full integration verification completed with actual CSV/GeoJSON browser downloads and import workflows, isolated SQLite preview/commit/rollback checks, Vite proxy, metadata and map/list refresh. See `V2.4_HANDOFF.md` for results and remaining limits.
- [x] V2.4G: Independent QA audit PASSED. Full audit of V2.4A–V2.4F completed with 0 P0, 0 P1, 0 P2 defects and 1 P3 defect (missing `favicon.ico` 404). All 3 mandatory unresolved targets resolved (LineString rejection exercised in browser with screenshot, metadata validation permutations exercised in browser with screenshot, favicon 404 investigated and documented). Multi-viewport layouts verified via Chrome CDP. All Go and Vitest regressions pass 100%. V2.4H closure pending explicit authorization.
- [x] V2.4H: Fixed DEF-01 with a local SVG favicon referenced by `frontend/index.html`; verified HTTP 200 and no favicon fallback in Chrome on Vite dev and production assets. Backend tests/race/vet/build and 136 frontend tests/TypeScript/production build passed. At V2.4H phase close, work remained unstaged pending separate authorization; V2.4 was subsequently finalized in commit `8486b20207b68bcf5bb115b6aa414630f4094442`.

## V2.5 planning status

- [x] V2.5A source-backed architecture audit and implementation plan completed in `V2.5_TECHNICAL_PLAN.md`; handoff recorded in `V2.5_HANDOFF.md`.
- [x] Verified baseline: `8486b20207b68bcf5bb115b6aa414630f4094442` on `master`; starting worktree clean.
- [x] V2.4 is committed; its reported test/QA evidence remains historical and was not rerun during this planning phase.
- [x] V2.5B implemented additive migration 004 for geofence definitions only, circle and simple Polygon validation/containment, and additive definition CRUD/list APIs. No frontend or membership/event behavior was implemented. Final completion check: `go build ./...` and `git diff --check` passed; full Go tests, race tests and vet passed.
- [x] Migration 004 creates only the geofence definition table and its listing index. Geometry limits and known limitations are recorded in `V2.5_HANDOFF.md`; a 256-vertex geometry microbenchmark is also recorded there.
- [x] V2.5C adds migration 005 for current membership and append-only event snapshots, atomic entity coordinate transitions, silent definition lifecycle recalculation, membership/event history APIs, migration backfill for pre-existing enabled definitions, rollback/concurrency tests, and SQLite performance measurements. Go test/race/vet/build and diff hygiene pass; see `V2.5_HANDOFF.md` for exact results and limitations.
- [x] Original take-home repository was not modified; persistent SQLite database was not opened. No dependency was added.
- [x] V2.5D implemented full frontend geofence operations: GeofenceManagerModal (list, events, create/edit tabs), Leaflet vector overlays (Circle & Polygon, independent from clustering), EntityDetail Geofences tab with authoritative containment states and evaluated coordinate notices, historical transition event timeline (ENTER/EXIT badges, keyset pagination, filters, semantic disclaimer), 26 test files / 165 Vitest unit tests passing (100%), production build passing, and automated headless Chrome CDP verification across desktop, tablet, and mobile viewports with 6 screenshots captured.
- [x] V2.5E full integration verification, performance profiling, and independent QA audit recorded a historical PASS (0 P0/P1). A later release review reported a geofence deletion blocker; follow-up did not reproduce it and added FK/API/repository evidence. Final Go test/race/vet/build, frontend tests/typecheck/build, and whitespace checks passed on 2026-11-03; source review recorded one P2 styling mismatch (details in `V2.5_HANDOFF.md`). V2.5 was committed locally as `43ce1a7d51950a6cc0ec79692cc3b779b252340b`; no release occurred.

## Streamlined delivery plan: V2.6–V2.8

- **V2.6 — Security & Reliability:** V2.6A–D are complete. Planning is preserved in `V2.6_TECHNICAL_PLAN.md`; implementation, QA, final Git evidence, and operating guidance are in the V2.6 report, handoff, QA, and operations documents. Production deployment readiness remains subject to operational verification.
- **V2.7 — Essential IoT Monitoring:** V2.7A–D are complete. The original V2.7C FAIL, targeted remediations, and independent responsive re-verification PASS remain recorded without rewriting history. V2.7D created one local commit after final backend/frontend verification. No push, tag, release, or deployment occurred.
- **V2.8 — Final Integration, QA, and Release Closure:** V2.8A automated/isolated integration QA is complete; browser verification was unavailable and is excluded from that verdict. V2.8B follow-up work and release closure require separate authorization. Demonstrate restore, repeat any remaining security/performance checks, and finalize deployment/recovery docs without another architecture overhaul.

**Authorization boundary:** V2.6A–D and V2.7A–D are recorded. The original V2.7C QA failure, remediation, and scoped independent responsive PASS remain in the evidence trail. V2.7D local finalization and V2.8A QA are complete. V2.8B, push, tag, release, and deployment remain separate future work requiring explicit authorization.

## V2.7A Planning Closure

- [x] Verified baseline on `master`: HEAD `9d7871e50f6d59befcffe675054e2d5fb373ef5c`, subject `feat: complete GeoEntity V2.6 security and reliability`, parent `43ce1a7d51950a6cc0ec79692cc3b779b252340b`, author `Muhamad Fauzan <vanz66hu@gmail.com>`. Initial worktree contained partial V2.7A documentation changes in these three files; no application source changes were present.
- [x] Recorded repository-backed auth, entity status/deletion, migration, SQLite concurrency, backup, API, and frontend constraints in `V2.7_TECHNICAL_PLAN.md`.
- [x] Proposed device credentials, migration 007, idempotent telemetry, history APIs, monitoring dashboard, security/rate limits, retention, performance assumptions, and test gates.
- [x] Created `V2.7_HANDOFF.md`; preserved V2.6 reports and history.
- [x] Planning only: no source, dependency, runtime configuration, migration, or database change; no tests/migrations run; no staging or commit.
- [x] At V2.7A planning close, V2.7B required separate explicit authorization; the owner later authorized and completed V2.7B as recorded below.

## V2.7B Implementation Closure

- [x] Migration 007 adds only device registry, credential, sensor unit, and observation tables with indexes, checks, restrictive references, unit/scope enforcement, and append-only triggers. No device membership/event persistence was added; migrations 001–006 are unchanged.
- [x] Device credential lifecycle CLI, one-time random secrets/hash-at-rest, transactional audit, exact device-authenticated ingestion, transaction-bound credential recheck, idempotency, and transactional entity deletion detachment are implemented.
- [x] Admin-only bounded devices/latest/history APIs, keyset history pagination, single-process API lock/rate limits, schema-7 backup handling and schema-6 restore compatibility, and monitoring dashboard are implemented.
- [x] `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, frontend tests (29 files/172 tests), and frontend production build passed. One local synthetic 10k benchmark is recorded in the handoff/report.
- [x] Persistent SQLite database and `[LOCAL_PATH_REDACTED]` were not accessed. No files were staged, committed, pushed, tagged, or released.
- [x] V2.7C independent QA executed. Verdict FAIL: one P1 entity deletion defect and two P2 findings; details in `V2.7C_QA_REPORT.md`. Browser QA was unavailable, so no browser interaction pass is claimed.
- [x] V2.7B targeted remediation addresses the three V2.7C findings with regression coverage; see `V2.7B_REMEDIATION_REPORT.md`. The historical V2.7C FAIL report remains unchanged; remediation checks are not a substitute for an independent QA rerun.
- [x] V2.7C re-test report records all three findings and automated regression gates passing, with Chrome browser QA evidence. Final Git audit found the reported 390×844 screenshot shows navigation overlap/clipping despite the responsive PASS claim; see the audit hold addendum in `V2.7C_RETEST_REPORT.md`.
- [x] V2.7B targeted responsive remediation reproduced and fixed the 390×844 Sign out/API badge overlap and clipped header navigation. Chrome geometry, visibility, overflow, and click-path checks pass at 390×844, 768×1024, and 1440×900; frontend tests/build pass. Evidence is in `V2.7B_RESPONSIVE_REMEDIATION_REPORT.md`.
- [x] Independently re-verify the responsive remediation in Chrome at 390×844, 768×1024, and 1440×900, including header geometry, keyboard/pointer operation, monitoring dashboard data, filtering, pagination, empty/loading/error states, and logout. Evidence is appended to `V2.7C_RETEST_REPORT.md`.
- [x] V2.7D final audit: backend tests/race/vet/build and frontend tests/build passed; diff hygiene passed; only reviewed V2.7 deliverables were staged. The V2.6 parent and corrected author identity were verified, and exactly one local V2.7 commit was created. Persistent database and `[LOCAL_PATH_REDACTED]` remained untouched. No push, tag, release, deployment, or V2.8 work occurred; see `V2.7_HANDOFF.md` for details and known limits.

## V2.8A Final Integration & QA Closure

- [x] Verified expected baseline `master` at `a00c11c286b93bf3c3a7666320343421496da1d2`, subject `feat: complete GeoEntity V2.7 IoT monitoring`, parent `9d7871e50f6d59befcffe675054e2d5fb373ef5c`, author `Muhamad Fauzan <vanz66hu@gmail.com>`; starting worktree was clean.
- [x] Backend Go tests, uncached race suite, vet and build passed. Frontend test script passed (29 files/172 tests); TypeScript/Vite production build passed with a non-blocking bundle-size advisory.
- [x] Existing automated tests passed for auth/session boundaries, entity/device lifecycle and preserved telemetry, input validation/idempotency, latest/history pagination and filters, query indexes, backup/restore schema compatibility, and V2.5 API regressions.
- [x] Browser automation was unavailable; no V2.8A screenshots or responsive/browser interaction claim is made. Browser scope is explicitly excluded from the PASS verdict. Earlier V2.7 browser reports remain historical evidence.
- [x] QA found no new P0/P1/P2 defect in the automated and source-review scope. Known single-process SQLite/rate-limit, indefinite-retention, bundle-size, and untested scale/recovery limits are recorded in `V2.8A_QA_REPORT.md`.
- [x] Only `V2.8A_QA_REPORT.md`, `V2.8_HANDOFF.md`, and this roadmap were changed. No source, tests, dependency, migration, database, or runtime configuration was modified; nothing was staged or committed.
- [x] Persistent `backend/data/geoentity.db` and `[LOCAL_PATH_REDACTED]` were not accessed. No push, tag, release, deployment, or V2.8B work occurred.
