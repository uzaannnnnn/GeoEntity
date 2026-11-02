# GeoEntity V2 — Product Roadmap

**Status:** V2.0 planning complete. V2.1 foundation implementation and independent QA are complete; V2.1E final Git closure is authorized. V2.2 requires explicit authorization.

## Product direction

Evolve the current single-user GeoEntity CRUD map into a spatial asset and IoT operations platform while retaining the React/TypeScript, Go, Leaflet, and SQLite foundation. Keep the existing API and entity data usable through additive changes. Do not introduce services, infrastructure, or dependencies until a roadmap sprint requires them.

## Workspace and current system inventory

The active V2 workspace is `./`, a separate Git repository from the original take-home project at `[LOCAL_PATH_REDACTED] The active V2 repository is on branch `master`, based on `acfa9a6b235738563a778e29c28f1feb793e4c6a`. The original take-home directory was not modified.

The V2 copy has a local, Git-ignored `backend/data/geoentity.db` with two entity rows, a V1 baseline in `schema_migrations`, and `PRAGMA user_version = 0`. A timestamped backup was independently verified before baseline registration; preserve both data and backup. Neither belongs in Git.

| Area | Implemented now | V2 gap |
|---|---|---|
| Map | Leaflet/React Leaflet, configurable official OSM raster URL, attribution, zoom, custom entity markers, marker selection, auto-fit on initial/manual refresh, draft location click/drag | No clustering, locate-me, fit-all control, legend, coordinate inspector, or basemap switcher |
| Entities | List, simple name/description search, type/status filters, details, create/update/delete, UUIDs, UTC timestamps | No tags/categories/custom attributes, advanced server-side search, bulk import/export, or timeline |
| Geospatial | Latitude/longitude validation and map visualization | No geometry model, geofence membership, spatial event history, or geometry test package |
| IoT | Generic entity records can represent devices | No telemetry, sensor history, threshold alerts, ingestion, or simulator |
| Backend | Go `net/http`, `database/sql`, repository abstraction, SQLite, JSON REST CRUD, input validation and error envelopes | No authentication/authorization, pagination, rate limiting, OpenAPI document, or production deployment config |
| Database | `entities` plus `schema_migrations`; ordered embedded migration runner, SHA-256 checksums, V1 recognition | No feature tables for tags, activity, geofences, or telemetry |
| Quality | Backend handler/persistence tests; frontend API/validation/formatting/state/navigation unit tests; TypeScript/Vite build | No browser E2E or map interaction harness; no import/geofence/telemetry tests yet |

### Current implementation quality

The current code is a small, understandable monolith with clear HTTP handler, entity validation, and SQLite repository boundaries. API error envelopes are consistent, invalid entity input is rejected independently by the backend, and frontend mutations apply their successful response before reconciling the list. This is a sound base for incremental evolution.

The V2.1 runner now handles persistence evolution: numbered migrations are ordered, checksummed, and recorded transactionally. V1 recognition checks the legacy table, constraints, columns, and index. `PRAGMA user_version` intentionally remains zero; `schema_migrations` is authoritative.

The entity list endpoint currently loads all rows. This is acceptable for the take-home dataset but must be bounded before bulk entity counts or telemetry are introduced. The Go repository limits the SQL pool to one open connection, which serializes access and suits this MVP but will constrain concurrent ingestion and reads.

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

1. **Custom attributes, tags, and categories**
   - Define whether attributes are per entity type or per entity; define allowed JSON types, size limits, validation, and filter semantics before schema work.
   - Keep the existing `type` field as-is; categories/tags are separate concepts.
   - Acceptance: old entity requests still succeed; new attributes are validated on both client and server; tag/category relations do not duplicate.
2. **Advanced search and filtering**
   - Add server-side query parameters with deterministic ordering and cursor/limit bounds. Preserve the existing `{data: [...]}` response shape and add optional top-level metadata only.
   - Acceptance: combined filters, empty result, malformed filter, page boundaries, and stable cursor behavior are covered.
3. **CSV and GeoJSON import/export**
   - Support preview/validation before writing. Validate every row/feature and all references before opening the write transaction. Initial commit mode should be all-or-nothing with a documented duplicate-ID policy and file/row limits.
   - Acceptance: valid import, mixed-invalid preview, invalid file rejection, duplicate handling, transaction rollback, export round-trip, and data-limit behavior are tested.
4. **Activity timeline**
   - Append events for entity create/update/delete and completed imports. Events are append-only; corrections are new events, never edits to history.
   - Acceptance: a failed entity write creates no success event; a successful write and its event commit atomically; timeline paging is stable.

### Level 3 — Geospatial Intelligence

1. **Circle and polygon geofences**
   - Define GeoJSON coordinate order as `[longitude, latitude]`; support a deliberately bounded geometry subset first (circle and Polygon; decide holes/MultiPolygon separately). Document boundary inclusion and antimeridian behavior.
   - Acceptance: geometry validation and containment tests cover boundary, outside/inside, malformed ring, degenerate polygon, holes if enabled, antimeridian, and high-latitude cases.
2. **Membership and entry/exit detection**
   - Evaluate membership only after a persisted coordinate change. In one transaction, update the entity, compare prior and new zone states, update membership, and append transition events. Repeated identical coordinates must not emit duplicate events.
   - Acceptance: retries are idempotent; concurrent updates cannot create contradictory transitions; a failed transaction leaves no partial membership/event writes.
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
| V2.2 — Map Intelligence | Fit-all, locate-me, legend/coordinate inspector, clustering; basemap only after policy decision | V2.1; existing map | Manual/browser tests and marker-scale test pass; OSM attribution/provider compliance documented |
| V2.3 — Entity model and activity | Custom-attribute semantics, tags/categories, append-only mutation activity, safe additive schema | V2.1 | Old CRUD clients work; new data validated; mutation+activity atomicity tested |
| V2.4 — Search and bulk data | Server filtering/pagination, CSV/GeoJSON preview/import/export, atomic commit | V2.3 | Full validation precedes transaction; rollback, limits, duplicate policy, round-trip tests pass |
| V2.5 — Geospatial intelligence | Geometry contract/library decision, circle/polygon validation, membership/event transitions, history | V2.3; telemetry/auth not required for local single-user evaluation | Pure geometry tests plus transaction/idempotency tests pass |
| V2.6 — Security gate | Authentication, authorization boundaries, rate limits, secrets, audit actor identity, deployment threat model | Before any external API exposure; can be moved earlier | Anonymous access and role matrix are tested; no external write endpoint is exposed without protection |
| V2.7 — IoT monitoring | Telemetry model/ingestion, history, battery/signal and thresholds, demo simulator if explicitly enabled | V2.1, V2.6 for external use | Ingestion is bounded/idempotent/protected; demo data is clearly labeled and disabled by default |
| V2.8 — Enterprise evaluation | Production monitoring, analytics requirements, SQLite/PostGIS benchmarks and migration decision | Measured workload from prior sprints | Written benchmark and go/no-go decision; no database replacement without evidence |

V2.3 and V2.2 can be planned in parallel by the human team, but execution should still use reviewed interfaces and migrations. Database foundation is a prerequisite for every backend feature sprint.

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
