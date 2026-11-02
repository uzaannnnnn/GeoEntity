# GeoEntity V2 — Product and API Contract Proposal

**Status:** V2 product contract proposal for future feature sprints. Existing `/api` behavior and the V2.1A migration baseline are implemented compatibility constraints; the proposed V2 feature APIs still require sprint authorization.

## 1. Contract principles

1. Preserve the current entity model, UUIDs, timestamps, CRUD routes, status codes, success/error envelopes, and validation.
2. Keep entity coordinates in current REST fields as decimal-degree `latitude` and `longitude`; GeoJSON coordinates are always `[longitude, latitude]`.
3. Backend validation is authoritative. New bulk writes validate the complete file before any DB transaction.
4. Use additive APIs/migrations first. No silent data loss, destructive migration, or automatic reset of the existing SQLite file.
5. Activity and telemetry records are append-only. Correct errors by adding compensating records, never rewriting history.
6. Geofence event generation is deterministic and testable, not presented as GPS real-time tracking.
7. No telemetry ingestion is exposed externally until auth, authorization, request limits, rate limits, and audit identity exist.

## 2. Current entity and API baseline

### Entity V1

`id` UUID; `name` required/non-empty after trim; `type` one of `vehicle`, `iot_device`, `facility`, `other`; `description` optional; `status` one of `active`, `inactive`, `maintenance`; finite latitude in `[-90,90]`; finite longitude in `[-180,180]`; UTC `created_at` and `updated_at`.

### Existing endpoints (must remain valid)

| Method/path | Current behavior |
|---|---|
| `GET /api/health` | `200 {"data":{"status":"ok"}}` |
| `GET /api/entities` | `200 {"data":[entity,...]}`; currently unpaginated, stable `created_at,id` ordering |
| `GET /api/entities/{id}` | `200 {"data":entity}`, `400` invalid UUID, `404` absent entity |
| `POST /api/entities` | Strict JSON input, `201 {"data":entity}` |
| `PUT /api/entities/{id}` | Full replacement of writable fields, `200 {"data":entity}` |
| `DELETE /api/entities/{id}` | `204` with empty body |

Error form remains `{"error":{"code":"...","message":"...","field":"..."}}`; `field` may be omitted. Existing decoder rejects unknown properties. New fields require backend support before the frontend sends them.

## 3. Proposed V2 entity extensions

These types/semantics need product approval before migrations:

| Concept | Proposal | Validation/compatibility |
|---|---|---|
| Tags | User-managed string/tag catalog with normalized unique name and many-to-many entity link | Preserve legacy `type`; define case/space normalization and max length; duplicates are idempotent |
| Categories | Separate managed taxonomy, not a replacement for `type` | Decide if one or many categories per entity; initial recommendation: many-to-many for consistency with tags |
| Custom attributes | Key/value attributes with an explicit type/schema policy; bounded strings/numbers/booleans/enums | Reject reserved keys (`id`, `name`, coordinates, timestamps); define limits and type coercion; no arbitrary executable/query expressions |
| Activity | Append-only entity mutation events with action, entity ID, UTC timestamp, actor ID once auth exists, and versioned bounded change payload | Insert in same transaction as source mutation; system actor allowed only in single-user phase |
| Geofence | UUID, name, enabled flag, geometry kind, geometry data, created/updated times | Geometry validated and normalized server-side; coordinate/geometry size and vertex count limited |
| Geofence membership | Current inside/outside state per entity-zone plus last evaluation version/time | Derived state; updated only when coordinates or geofence definition change |
| Geofence event | Append-only enter/exit transition with entity/zone IDs, evaluated coordinates/time, and source | Emit only on membership transition, not every update; atomically update state and event |
| Telemetry | Append-only measurement with entity ID, metric, typed value/unit, measured time, received time, source, optional source event ID | Bounds, allowed metrics, units, event-time skew, idempotency, and retention required before ingestion |

For custom attributes, choose either a JSON object on the entity or normalized definition/value tables after deciding whether server-side filters, required fields, and per-type schemas are in scope. Initial recommendation: normalized tables if filtering by custom keys is required; otherwise one bounded validated JSON object is simpler. Do not mix both approaches without a migration plan.

## 4. Proposed API additions

All paths below are candidates and require review. Existing endpoints remain unchanged.

### Search and pagination

`GET /api/entities?query=&type=&status=&tag=&category=&limit=&cursor=`

- `limit` has a documented default and maximum.
- Cursor is opaque and tied to deterministic sort keys; reject malformed cursors.
- Preserve `data` as an array; add optional top-level `meta: {next_cursor, limit}` so existing clients reading `data` remain compatible.
- Search behavior (substring vs token/prefix, case, Unicode) must be specified before implementation.

### Entity metadata

- `GET /api/tags`, `POST /api/tags`, `GET /api/categories`, `POST /api/categories`.
- `GET /api/entities/{id}/tags`, `PUT /api/entities/{id}/tags` (replace set only if this explicit semantic is chosen).
- `GET /api/entities/{id}/activity?limit=&cursor=`.
- Keep CRUD response fields additive and old clients functional.

### Import/export

- `POST /api/imports/entities/preview`: multipart `file` plus `format=csv|geojson`. No writes. Return detected format, accepted/rejected counts, bounded normalized sample, and row/feature errors.
- `POST /api/imports/entities`: validate the entire bounded upload, then write in one transaction. Initial mode is `create_only`; an existing ID or duplicate in-file key is rejected, not overwritten. A future upsert mode must be explicit and permissioned.
- `GET /api/entities/export?format=csv|geojson&...`: streamed file response with filename, content type, filters, and data schema version.
- Limit file bytes, rows/features, per-value size, and preview response size. Return `413` if size limits are exceeded. Validation errors use existing envelope plus optional `details: [{row, field, code, message}]`.
- GeoJSON export uses FeatureCollection; each entity is a Point with `[longitude, latitude]`. Define property mapping and ID representation in the finalized contract.

### Geofences

- `GET/POST /api/geofences`, `GET/PUT/DELETE /api/geofences/{id}`.
- `GET /api/geofences/{id}/events?limit=&cursor=`.
- Proposed circle representation: center Point in GeoJSON order plus `radius_m`.
- Proposed polygon representation: GeoJSON Polygon. Initial release should avoid MultiPolygon unless required. Decide holes, ring winding/closure, antimeridian and high-latitude behavior, and boundary inclusion before accepting geometry.
- A coordinate update should calculate prior and next membership, persist new coordinates/membership, and append zero or more enter/exit events in one SQLite transaction. Identical coordinates must not duplicate events.

### Telemetry

- Proposed `POST /api/telemetry/ingest` accepts bounded batches; `GET /api/entities/{id}/telemetry?metric=&from=&to=&limit=&cursor=` returns bounded historical windows.
- Include `source` and optional source event ID; make retries idempotent.
- External ingestion requires authenticated identity/API key, authorization to entity, rate limits, maximum batch/body, timestamp-skew policy, and auditing.
- Simulator routes, if approved, must be development/demo-only and label every reading `source=demo`.

## 5. Database evolution proposal

### Existing baseline

The V1 baseline has one `entities` table with CHECK constraints for current types/statuses and coordinate range, a unique text UUID primary key, and a creation-order index. Migration 001 remains embedded. V2.1A added the ordered migration runner and ledger; the local ignored database now has its two preserved rows and a V1 baseline ledger. `PRAGMA user_version` remains 0 and is not the application's schema history.

### Target tables (proposal)

- `schema_migrations(version, name, checksum, applied_at)`.
- `tags`, `entity_tags`, `categories`, `entity_categories` as taxonomy requires.
- `attribute_definitions` and/or `entity_attribute_values` if typed custom attributes/filtering is approved.
- `activity_events` (append-only).
- `geofences`, `geofence_memberships`, `geofence_events` (events append-only).
- `telemetry_records` (append-only, time/entity/metric indexed).

Avoid unnecessary columns on `entities` and avoid changing its existing CHECK constraints until a tested table rebuild is justified. Keep current position on `entities`; if history is needed, telemetry or a separate position-history table is the source for it.

## 6. Migration and backward-compatibility procedure

1. Back up the DB and verify backup restore before migration.
2. Detect existing `entities` table and required columns. If valid legacy schema and no migration ledger, register V1 baseline; do not treat `user_version=0` as an empty DB.
3. Apply ordered migrations one time; fail closed on unexpected schema. Never drop/recreate the DB to “upgrade.”
4. Prefer adding new tables/indexes. For unavoidable SQLite table rebuilds, preserve indexes, constraints, IDs, data, and old reads/writes in a fixture test.
5. Deploy backend accepting old payloads and new optional fields before frontend emits them.
6. Keep old routes/response envelope; add endpoints. Add `/api/v2` only for an approved breaking change with a transition period.
7. Test current V1 API contract after every migration; test migration against the two-row legacy fixture and a populated synthetic fixture.

### V2.1A implementation record

- `schema_migrations` records sequential migration version, exact filename, SHA-256 checksum, and UTC application timestamp. Each SQL migration and its ledger insert commit in one transaction.
- The runner rejects sequence gaps, unknown migration files, unknown/unexpected un-ledgered schema objects, malformed ledger schema, missing ledger versions, filename mismatches, and checksum mismatches.
- Legacy V1 baseline recognition checks the exact `entities` SQL definition (including CHECK constraints), ordered column metadata, and the required `idx_entities_created_at` definition. Valid V1 databases are baselined without rewriting rows. Empty databases execute migration 001 through the same migration path.
- Temporary database tests cover empty initialization, a synthetic two-row V1 fixture, baseline registration and row preservation, rerun/reopen, checksum mismatch, unknown schema, missing index/constraint, failed migration rollback, and ledger consistency. Existing handler tests cover V1 health and CRUD routes, payloads, response envelopes, validation, status codes, and persistence.
- The actual local database was backed up before migration with SQLite's backup API to `[LOCAL_PATH_REDACTED] Support/GeoEntity/backups/geoentity-20261102T135501Z.db`. It was independently opened read-only; schema and both records matched. After startup migration, the exact entity rows still match the backup and version 1 is recorded. Backup SHA-256: `c847028ba472bfd29b92b319ec4afadabaf5a6a709d5648d170f1f245c886acf`.
- Restore only while the API is stopped: copy the backup over `backend/data/geoentity.db`, then restart. Do not add the backup or local database to Git.

## 7. Error, validation, and transaction rules

- Keep current errors: `INVALID_JSON`, `INVALID_ID`, `VALIDATION_ERROR`, `NOT_FOUND`, `METHOD_NOT_ALLOWED`, `INTERNAL_ERROR`.
- Use `413` for file/body limit, `409` for conflicts/idempotency policy, and `429` for rate-limit responses if/when those controls are implemented.
- Validate full import input before beginning a write transaction. The commit transaction inserts entities, links, and activity records atomically.
- Validate GeoJSON structure and coordinate ranges before geometry calculations. Geometry parsing errors return a field/feature reference, never raw stack/database output.
- Validate telemetry event time, metric, units, source, batch bounds, idempotency, and entity authorization before write.

## 8. Data and lifecycle rules

- Entity IDs remain immutable UUIDs; `created_at` is immutable; `updated_at` changes on a successful current-state update.
- Entity deletion must define history behavior before adding FK cascades. Recommended: preserve activity/geofence events with a tombstone or nullable reference rather than cascading historical rows away.
- Activity and telemetry are append-only. Corrections use correction events/new readings.
- Imports default all-or-nothing and create-only. Partial success or upsert requires an explicit product decision and separate acceptance tests.
- Telemetry retention, aggregation, export, and deletion must be agreed before real ingestion. Do not leave indefinite time-series retention implicit.

## 9. Security and source-of-truth boundaries

- Backend is authoritative for validation, IDs, timestamps, persistence, membership, and generated alerts.
- Frontend map state is ephemeral; it never replaces the backend as source of truth.
- Current API is unauthenticated. Keep it local/trusted-network only until auth and API protection are implemented.
- Any demo simulator must be visibly labeled and data-tagged. Do not claim live tracking unless an actual device/source sends timestamped coordinates.

## 10. Dependency policy

- No dependencies are added during V2.0 planning.
- For clustering, evaluate `react-leaflet-cluster`; current project docs claim React 19 and React Leaflet 5 compatibility, matching the lockfile, but confirm exact release/peer deps, maintenance, license, CSS, and custom markers in implementation sprint. [Project](https://github.com/akursat/react-leaflet-cluster)
- For GeoJSON/geometry, prototype `github.com/paulmach/orb` against required point, polygon, holes, boundary, antimeridian, and projected/geodesic tests before adoption. It exposes GeoJSON and geometry subpackages, but the project must prove the specific containment behavior. [Project](https://github.com/paulmach/orb)
- Use Go standard `encoding/csv`, `encoding/json`, browser geolocation, and existing Leaflet controls where sufficient.
- No Postgres/PostGIS dependency is selected. Define measurable thresholds and a separate migration project first.

## 11. Acceptance criteria for future implementation

- V1 API clients continue to create/read/update/delete the same entity fields and receive the same envelopes/statuses.
- Upgrade from existing DB with two rows preserves IDs and values, is repeatable, and leaves a verifiable migration ledger.
- Search and pagination are bounded and stable.
- Import preview never writes; commit either writes the full valid import plus event records or writes nothing.
- Activity and geofence events cannot be updated/deleted through application APIs.
- Geofence geometry has documented semantics and deterministic edge-case tests; coordinate update and event state commit atomically.
- Telemetry endpoints are protected before external exposure, batch-limited, idempotent, timestamp-aware, and separately retained.
- Basemap URLs, attribution, caching/usage policy, and provider terms are documented and reviewed at implementation time.
