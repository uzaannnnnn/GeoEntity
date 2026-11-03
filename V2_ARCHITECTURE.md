# GeoEntity V2 — Architecture and Evolution

> **Historical architecture plan:** sections below preserve the V2.0/V2.1 baseline and proposed target architecture. The security warning and “existing architecture” diagram reflect that earlier baseline, not the current application. Current auth, device, geofence, and monitoring behavior is summarized in `README.md`; verify details in source before relying on this proposal.

**Document status:** V2.0 architecture baseline with V2.1 implementation notes; future product architecture remains a proposal until its sprint is authorized.

## Workspace boundary and baseline

- V2 planning workspace: `./`.
- Original take-home repository: `[LOCAL_PATH_REDACTED]` It was not modified.
- The two directories have separate `.git` directories but currently share base commit `acfa9a6b235738563a778e29c28f1feb793e4c6a`.
- V2 repository branch: `master`; tracked status clean at audit; no remote configured.
- The ignored local SQLite database has two entity rows, a V1 baseline in `schema_migrations`, and `PRAGMA user_version = 0`. It was backed up and verified before the V2.1A baseline registration; both rows were preserved.

## Existing architecture inventory

```text
React 19 + TypeScript + Leaflet/React Leaflet
      │ JSON fetches under /api
      ▼
Go net/http handler ──► entity validation ──► SQLite repository
      │                                      modernc.org/sqlite
      └── no auth/CORS middleware or service layer
```

### Frontend

- `frontend/src/App.tsx` owns entity list/selection, forms, map draft location, status, API errors, and mutation reconciliation.
- `frontend/src/api/` has a centralized Fetch wrapper and typed endpoint calls.
- `frontend/src/components/MapView.tsx` owns Leaflet markers, popups, map click handling, bounds, and map navigation.
- `frontend/src/utils/` holds validation, formatting, local entity-state operations, and map navigation.
- Current functional scope: list/search/filter, details, create by map click or manual coordinates, edit, delete, status/type markers, initial/manual bounds, refresh, health indicator, loading/empty/error states.
- Current test scope: API client, validation, formatting, entity-state helpers, and navigation helper. There is no browser E2E harness in the repository.

### Backend

- `backend/cmd/api/main.go` starts one configurable HTTP server and opens SQLite.
- `backend/internal/handler/api.go` implements health and entity CRUD routes; it applies a 1 MiB request limit, rejects unknown JSON fields, maps validation errors, and hides internal errors.
- `backend/internal/entity/entity.go` defines the entity DTO and allowed type/status/coordinate rules.
- `backend/internal/repository/sqlite.go` uses `database/sql`, one open connection, ordered embedded SQL migrations with checksums and a ledger, stable list ordering, and CRUD queries.
- Driver: `modernc.org/sqlite`; UUID library: `github.com/google/uuid`. Frontend runtime dependencies: React, React DOM, Leaflet, and React Leaflet. Development dependencies include Vite, TypeScript, and Vitest.

## Existing data and migration design

Migration `001_create_entities.sql` remains the V1 baseline. V2.1A adds an ordered embedded migration runner and `schema_migrations(version, name, checksum, applied_at)`. It orders numbered files, rejects gaps and unknown filenames, verifies checksums and ledger order, and applies each migration and its ledger entry in one transaction. Existing `user_version=0` is not treated as an empty database.

For a database without a ledger, the runner recognizes V1 only when the table definition, columns, CHECK constraints, and `idx_entities_created_at` match the embedded baseline. It registers version 1 without rewriting rows. A genuinely empty database gets version 1 by executing that same baseline migration. Unexpected tables or schema fail closed. Applied migration names and SHA-256 checksums are verified on every startup.

The actual local V1 database was upgraded after a verified external backup. Its two rows matched the backup exactly afterward, and the ledger contains version 1. `PRAGMA user_version` remains 0; `schema_migrations` is the authoritative application schema history.

### Safe migration path

1. Before any future schema change, use SQLite's backup API or a stopped-process file copy to create and independently verify a timestamped backup outside Git.
2. Add numbered migration files only; the runner verifies ordering, names, checksums, and ledger consistency.
3. Preserve existing rows and IDs. The V1 baseline is registered without rewriting records; unknown legacy schemas fail closed.
4. Apply new tables/indexes in additive migrations. Prefer side tables (`entity_tags`, `entity_attributes`, `activity_events`, `geofences`, `telemetry_records`) so the constrained legacy entity row and old CRUD path remain stable.
5. Keep tests isolated in temporary databases; test empty, exact V1, populated V1, checksum mismatch, rollback, reruns, and reopen before touching the local DB.
6. Keep migrations transactional and short. Import parsing/validation, geometry calculations over bulk datasets, and telemetry batch normalization happen before acquiring the SQLite write transaction.

The V2.1A backup is `[LOCAL_PATH_REDACTED]` (verified read-only; two records and schema match; SHA-256 `c847028ba472bfd29b92b319ec4afadabaf5a6a709d5648d170f1f245c886acf`). To restore, stop the API, copy the backup over `backend/data/geoentity.db`, then restart the API. Never restore while the database is open; keep the backup outside Git.

Do not commit the existing `.db` file. Use a scrubbed SQL fixture or deterministic seed in tests.

### SQLite evolution

- Keep `entities` as the current-state aggregate: stable UUID, current coordinates, type/status, timestamps.
- Add `tags`, `categories`, and link tables only after their semantics are defined; do not overload `type` or `status` for user-defined values.
- Store custom attribute definitions and values in explicit tables or a bounded JSON document according to the chosen query/validation model. Do not add an opaque unindexed JSON blob if filtering is a requirement.
- Make `activity_events` and telemetry records append-only. Use separate indexes by `(entity_id, created_at)` and `(entity_id, metric, recorded_at)` as workload requires.
- Store a geofence's declared geometry and validation version separately from derived membership. Membership is current state; transitions are append-only events.
- Evaluate WAL, busy timeout, connection pool, and backup behavior only with tests and measured concurrent workloads; do not change database settings as a substitute for a volume benchmark.

## Proposed target architecture

Keep one Go monolith and one SQLite database for the next roadmap levels. Keep map rendering and ephemeral interaction in the browser. Add small backend packages at actual domain boundaries; do not create generic frameworks or microservices.

```text
Frontend
├── features/map          Leaflet layers, controls, basemap adapter
├── features/entities     list, detail, forms, tags, attributes
├── features/imports      upload, preview, row/feature errors
├── features/geofences    drawing/editing and event history
├── features/telemetry    history/alerts; clearly marked demo source
└── api/types             stable JSON DTOs and error handling
         │ same-origin /api
         ▼
Go monolith
├── handler/api DTOs      existing CRUD plus additive domain routes
├── entity                entity rules and use cases
├── importexport          CSV/GeoJSON parse, validate, export
├── activity              append-only entity audit records
├── spatial               pure geometry validation and membership
├── telemetry             bounded ingestion and query use cases
├── auth                  introduced before external access
└── repository            SQLite and ordered migrations
         ▼
SQLite (V2 initial) ──► possible PostgreSQL/PostGIS only after benchmark evidence
```

### Frontend boundaries

- Keep map-specific Leaflet objects inside `features/map`; expose typed props/events rather than passing Leaflet state through the application.
- Keep geospatial calculations out of React components. UI drawing emits GeoJSON; backend validation is authoritative.
- Keep API DTO types in one place and extend them in the same change as backend request/response structs.
- Avoid making the current top-level `App.tsx` a feature router for all future levels; introduce feature modules when the first new V2 feature is approved.

### Backend boundaries

- Preserve `net/http` and repository pattern.
- Keep parsing, validation, and authorization before repository transactions. The transaction should contain only consistency-critical writes.
- Use context deadlines and explicit body/batch limits for imports/telemetry.
- Keep append-only history writes in the same transaction as the mutation they describe.
- Add auth middleware before any externally reachable write APIs; current API is unauthenticated and should be treated as local/trusted-network only.

## API contract evolution

The current API is unversioned under `/api`, with `{data: ...}` success and `{error:{code,message,field?}}` errors. Inputs are strict (`DisallowUnknownFields`). Preserve existing routes and status semantics for current clients.

### Compatibility policy

- Keep `GET/POST /api/entities`, `GET/PUT/DELETE /api/entities/{id}`, and `/api/health` intact.
- Existing fields (`id`, `name`, `type`, `description`, `status`, `latitude`, `longitude`, timestamps) retain meanings and formats.
- New response fields are additive. Preserve `data` as an array for the list; optional metadata can be an additional top-level `meta` property.
- New request fields require coordinated backend-first rollout because old backend strict decoding rejects unknown properties. Existing clients omitting them must continue to work.
- Add separate endpoints for imports, tags, activity, geofences, and telemetry rather than changing CRUD method semantics.
- Introduce a versioned `/api/v2` only for a breaking change, with a deprecation window and compatibility tests; the product label “V2” alone does not require breaking every route.
- Keep machine-readable error codes stable. Bulk validation may add an optional `details` array without removing existing `code`, `message`, or `field`.

### Candidate route extensions

These are proposals, not approved endpoints:

| Capability | Candidate route | Notes |
|---|---|---|
| Filter/page entities | `GET /api/entities?query=&type=&status=&tag=&category=&limit=&cursor=` | Bounded limit; stable cursor using current `created_at,id` order; response remains `{data:[...],meta:{next_cursor}}`. |
| Tag/category catalogs | `GET/POST /api/tags`, `GET/POST /api/categories`; entity links under `/api/entities/{id}/tags` | Authorization required when multi-user; define duplicate/name normalization. |
| Entity activity | `GET /api/entities/{id}/activity?cursor=&limit=` | Append-only event log; newest/oldest order fixed and documented. |
| Import preview | `POST /api/imports/entities/preview` | Multipart CSV or GeoJSON; no database writes; return accepted count and row/feature errors. |
| Import commit | `POST /api/imports/entities` | Validate full upload again, then one bounded transaction; initial mode create-only/reject duplicates. |
| Export | `GET /api/entities/export?format=csv|geojson` | Stream with content disposition and explicit schema/coordinate semantics. |
| Geofences | `/api/geofences`, `/api/geofences/{id}`, `/api/geofences/{id}/events` | Geometry validation, membership transaction, and event history. |
| Telemetry | `POST /api/telemetry/ingest`, `GET /api/entities/{id}/telemetry` | Do not expose ingestion externally before auth, rate limits, and idempotency. |

## Database migration and compatibility

- Existing IDs and rows remain unchanged. No V2 migration may rewrite IDs or normalize coordinates in place without a separately approved migration plan.
- Use additive new tables for tags, categories, custom attributes, events, zones, and telemetry. Changes to existing CHECK constraints require a tested SQLite table-rebuild migration and a backup; avoid them by keeping new taxonomy separate.
- Migration files must be ordered, applied once, checked, and tested on the existing V1 schema. Migration down files are useful for reversible development changes but are not a substitute for a production backup/restore plan.
- Make API compatibility tests exercise the old JSON payloads after every migration.
- Rollout order: back up; deploy backend/migrations that accept both old and new shapes; deploy frontend; verify; only later remove old behavior if a deprecation plan exists.

## Geospatial semantics

- Normal entity API remains `latitude` then `longitude` to preserve its existing DTO.
- GeoJSON strictly uses `[longitude, latitude]` in WGS84 decimal degrees. Import/export adapters perform that conversion at the boundary.
- Circle radius is metres and containment uses geodesic distance.
- Polygon rules must specify ring closure, winding normalization, holes, boundary inclusion, maximum vertices, antimeridian handling, and supported geometry types. Do not assume a planar point-in-polygon operation is correct directly on longitude/latitude for all regions.
- Define a pure spatial package with no DB/UI dependency. Test boundaries, holes if supported, degenerate rings, invalid lat/lon, antimeridian, and high latitude. Use a geometry library only after a prototype confirms these semantics.

## Bulk import integrity

1. Enforce request size, row/feature count, and per-field limits before parsing all input into memory.
2. Parse all rows/features and normalize coordinates/enum values.
3. Validate every record and cross-record duplicate/reference rule outside a write transaction.
4. Return a preview with stable row/feature numbers and errors; preview never mutates state.
5. On commit, revalidate the submitted content (or validate a cryptographically bound server-side preview token), begin one transaction, write the complete valid set, and commit. Initial policy is all-or-nothing.
6. Append corresponding activity events in the same transaction.
7. Test rollback on a late constraint failure and enforce uniqueness in SQLite, not only in frontend code.

## Activity and geofence event integrity

- Entity activity is append-only. Store actor ID nullable only during the pre-auth single-user phase, action, entity ID, UTC time, and a versioned, size-limited before/after snapshot or explicit changed fields.
- Geofence events are a distinct append-only event stream with zone/entity IDs, prior/new membership, coordinate/time, and source. Do not mutate past event rows to repair errors.
- Persist entity coordinate update, calculated membership state, and entry/exit events atomically. Reprocessing the same update must not duplicate transitions.
- Define deletion semantics: retain events with a tombstoned entity reference or remove a foreign key dependency that would erase audit history. Do not cascade-delete append-only history silently.

## Telemetry architecture

- Telemetry is append-only and separate from current entity metadata. Candidate fields: `id`, `entity_id`, metric/key, numeric or typed value, unit, `measured_at`, `received_at`, source, optional source event ID, and quality/status.
- Define late/out-of-order reading policy, idempotency, retention, query windows, and aggregation before implementation.
- Sensor history queries must be time-bounded and indexed. Do not load all readings into the browser.
- A demo simulator is a separate, explicit producer with `source=demo`; display “Demo data” in the UI, disable it by default for deployed environments, and never use its readings in production alerts.

## Security and production boundary

The present backend has no auth, CORS, rate limiting, or production proxy configuration. Do not expose it directly to the public Internet. Before external deployment, add authentication, authorization, CSRF/origin strategy appropriate to the chosen session/token design, rate/request limits, secret management, TLS termination, audit identity, and security tests. Telemetry ingestion must not precede that gate.

## Dependencies

### Preserve current dependencies

- Go standard library for HTTP, JSON, CSV, SQL interface, testing, and most request handling.
- React, TypeScript, Vite, Leaflet, React Leaflet; no new framework or GIS service.
- Current `modernc.org/sqlite` and UUID packages remain until a measured need justifies replacement.

### Minimal candidates to evaluate when the relevant sprint is authorized

| Need | Candidate | Decision and caveat |
|---|---|---|
| Marker clustering | `react-leaflet-cluster` | Its project documentation lists React 19 and React Leaflet 5 support, matching the current stack, and requires two CSS imports. Confirm current release, license, maintenance, bundle cost, custom divIcon behavior, and SSR/build compatibility before adding. [Project README](https://github.com/akursat/react-leaflet-cluster) |
| Geospatial/GeoJSON utilities | `github.com/paulmach/orb` | Candidate only for a prototype: it provides Go geometry types, GeoJSON support, geodesic distance utilities, and planar/projected subpackages. Confirm exact polygon containment, hole, boundary, and projection semantics against V2 acceptance tests before adoption. [Project README](https://github.com/paulmach/orb) |
| Locate Me, fit, controls, legend | Browser APIs and existing Leaflet | No new package expected. |
| CSV and JSON/GeoJSON parse | Go `encoding/csv`, `encoding/json` | No parser dependency expected for bounded initial imports. |
| Tags, custom attributes, activity, telemetry API | Existing React/Go/SQLite | No dependency expected for initial data and API work. |
| PostgreSQL/PostGIS | Not selected | Evaluate separately from workload evidence; do not install in this phase. |

No dependency was added in PHASE 0.

## OSM basemap compliance (V2.1B)

In V2.1B, frontend map provider compliance was implemented and verified:
- **Tile URL template:** The legacy `{s}.tile.openstreetmap.org/{z}/{x}/{y}.png` template was replaced with the official standard endpoint `https://tile.openstreetmap.org/{z}/{x}/{y}.png`.
- **Configurability:** Map tile URL, attribution, and max zoom are configurable via environment variables (`VITE_MAP_TILE_URL`, `VITE_MAP_ATTRIBUTION`, `VITE_MAP_MAX_ZOOM`) and `MapView` props, falling back to official OSM defaults via `frontend/src/utils/mapConfig.ts`.
- **Visible attribution:** Visible OpenStreetMap attribution (`© OpenStreetMap contributors` linking to `https://www.openstreetmap.org/copyright`) remains active in the Leaflet control and unobstructed by adjacent panels, banners, or toast notifications.
- **Browser policy:** Standard browser `Referer` headers are maintained (no restrictive `no-referrer` policy); tile caching relies on standard HTTP caching without custom offline prefetching or cache interception; no `User-Agent` spoofing is introduced.
- **Service terms and SLA:** The official OpenStreetMap standard tile layer operates on donated infrastructure on a best-effort basis without an SLA. For higher-volume use or public production deployments, a commercial or self-hosted tile provider with guaranteed availability and appropriate terms of service must be configured. Multiple provider switching UI is deferred until the basemap selection feature is authorized.

## Risks

- Local ignored database contains data; accidental reset/delete would lose it.
- `PRAGMA user_version` remains zero; the application migration ledger is authoritative. A database without that ledger must still be schema-verified before baselining.
- API strict unknown-field decoding requires backend-first additions.
- SQLite serialized connection and full-list reads will not support unbounded telemetry/list growth.
- Geofence false positives/negatives can produce safety-impacting alerts; semantics and geometry tests are mandatory.
- Activity/history/telemetry tables grow without retention and indexes.
- At this historical V2.0 baseline, authentication was absent; later V2.6 work added administrator sessions and related protections.
- OSM standard tile availability and URL-policy constraints are separate from OSM data licensing; a basemap's data license does not grant unrestricted tile-service usage.

## Current-state correction — 2026-11-03

The V2.0/V2.1 architecture diagram and baseline descriptions above are historical, not current. The current code has administrator authentication and session/CSRF/Origin middleware, bounded process-local limits, and a separate device credential on the exact telemetry ingest route. This does not claim the service has been deployed or production-hardened. See `AGENT.md`, `README.md`, and `V2.6_SECURITY_OPERATIONS.md` for current constraints. Later implementation records supersede earlier proposals where they differ.
