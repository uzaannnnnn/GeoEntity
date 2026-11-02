# GeoEntity Management System

GeoEntity is a map-centric web application for managing real-world entities with geographic coordinates. Users can view entities on a Leaflet map, inspect details, create records by selecting a map location or entering coordinates, update records, and delete them. Entity data is stored in SQLite and accessed through the Go backend.

## Features

- Interactive OpenStreetMap map with entity markers and visible map attribution.
- Entity list, search, type/status filters, and detail panel.
- Create, update, and delete workflows backed by persistent SQLite data.
- Map-click location selection, draggable draft marker, and manual coordinate entry.
- Frontend and backend validation for required fields, supported types/statuses, and coordinate bounds.
- Loading, empty, error, and mutation feedback states.

## Technology stack

| Area | Technology | Reason for selection |
|---|---|---|
| Frontend | React 19 | Component model fits the map, list, details, and forms that share entity state. |
| Frontend language | TypeScript | Catches mismatches in API payloads and entity state during development. |
| Frontend tooling | Vite 6 | Provides the configured development server and a TypeScript-checked production build with a small setup. |
| Map | Leaflet + React Leaflet | Leaflet provides the interactive map; React Leaflet connects its map and marker lifecycle to React components. |
| Map tiles | OpenStreetMap | Supplies the no-key-required base map used by the MVP. Attribution is displayed on the map. |
| Backend | Go `net/http` | Implements the REST API with Go's standard HTTP server and no web framework dependency. |
| Database access | `database/sql` | Uses Go's standard SQL interface for the repository layer. |
| SQLite driver | `modernc.org/sqlite` | Connects SQLite through `database/sql` without requiring a CGO/native SQLite build setup. |
| Persistent storage | SQLite | Keeps the MVP's entity data in a simple local database file. |
| Frontend tests | Vitest | Runs the existing API, validation, formatting, state, and map-navigation unit tests. |

## Architecture

```text
React + TypeScript + Leaflet
          │ relative /api requests
          ▼
Vite proxy (development only) ──► Go net/http REST API
                                      │ validation
                                      ▼
                               SQLite repository
```

In production, serve the frontend and route `/api/*` through a same-origin reverse proxy to the Go API. The repository does not include a production hosting or reverse-proxy configuration. The backend does not enable CORS because the documented production arrangement is same-origin.

The frontend loads its entity list from the backend. Successful mutations update the current UI state from the API response immediately, then fetch the list again to reconcile it with persistent data. SQLite remains the source of truth.

## Project structure

```text
.
├── backend/
│   ├── cmd/api/main.go
│   ├── internal/entity/       # Entity model and backend validation
│   ├── internal/handler/      # HTTP routes, JSON responses, API tests
│   ├── internal/repository/   # SQLite persistence and embedded migration
│   ├── .env.example
│   └── go.mod
├── frontend/
│   ├── src/api/               # HTTP client and entity API methods
│   ├── src/components/        # Map, list, details, and forms
│   ├── src/utils/             # Validation, formatting, and entity UI state
│   ├── .env.example
│   └── package.json
├── AGENT.md
├── HANDOFF.md
├── MASTER_PROMPT.md
└── MVP_CONTRACT.md
```

## Prerequisites

- Go **1.27.3 or later** (the version declared by `backend/go.mod`).
- Node.js **20.x, 22.x, or 24+** and npm (supported ranges include the installed Vitest version). The frontend was verified with Node.js 25.9.0 and npm 11.12.1.
- Internet access in the browser for OpenStreetMap tiles and the fonts referenced by the frontend page.

## Installation and local development

Clone or copy this repository, then use two terminal windows.

### Backend

```sh
cd backend
go mod download
go test ./...
go run ./cmd/api
```

By default, the API listens on `http://localhost:8080`. The backend creates the parent directory for its database file on startup.

### Frontend

```sh
cd frontend
npm ci
cp .env.example .env
npm test
npm run dev
```

Vite serves the development frontend at `http://localhost:5173`. Its `/api` development proxy forwards requests to the backend at `http://localhost:8080`.

To build the frontend:

```sh
cd frontend
npm run build
```

This runs `tsc -b` followed by `vite build`; output is written to the ignored `frontend/dist/` directory. Vite's configured proxy is development-only. `npm run preview` serves the built frontend but does not provide a backend or production `/api` reverse proxy.

## Environment configuration

| Variable | Used by | Default | Purpose |
|---|---|---|---|
| `GEOENTITY_ADDR` | Go backend | `:8080` | HTTP listen address. |
| `GEOENTITY_DB_PATH` | Go backend | `./data/geoentity.db` | SQLite file path, relative to the backend process working directory. |
| `VITE_API_BASE_URL` | Frontend build | `/api` | API base URL. Keep `/api` for same-origin development and production proxy routing. |
| `VITE_PROXY_TARGET` | Vite development server | `http://localhost:8080` | Backend target for the Vite `/api` proxy. |

The example files are [backend/.env.example](backend/.env.example) and [frontend/.env.example](frontend/.env.example). Vite reads frontend `.env` files. The Go server does **not** load dotenv files; set backend variables in the process environment, for example:

```sh
cd backend
GEOENTITY_ADDR=:8080 GEOENTITY_DB_PATH=./data/geoentity.db go run ./cmd/api
```

If using a different backend address during development, set `VITE_PROXY_TARGET` in the Vite environment. Do not put secrets in frontend variables: Vite variables prefixed with `VITE_` are included in browser code.

## Database and persistence

SQLite is initialized at startup from the embedded schema migration in `backend/internal/repository/migrations/001_create_entities.sql`. The default database file is `backend/data/geoentity.db` when the backend is launched from the `backend/` directory. The `data/` directory and SQLite files are excluded from Git.

Entity IDs are UUIDs generated by the backend. `created_at` and `updated_at` are stored and returned in UTC. `updated_at` changes on successful updates. Data survives backend and browser restarts as long as the SQLite file is preserved.

## REST API

All routes are rooted at `/api`. Request bodies use JSON. Successful entity responses use `{"data": <entity>}`; lists use `{"data": [<entity>, ...]}`. Errors use `{"error":{"code":"...","message":"...","field":"..."}}`; `field` is omitted when it does not apply.

| Method | Path | Success response |
|---|---|---|
| `GET` | `/api/health` | `200`, `{"data":{"status":"ok"}}` |
| `GET` | `/api/entities` | `200`, entity array; empty database returns `[]` |
| `GET` | `/api/entities/{id}` | `200`, entity object |
| `POST` | `/api/entities` | `201`, created entity object |
| `PUT` | `/api/entities/{id}` | `200`, updated entity object |
| `DELETE` | `/api/entities/{id}` | `204`, empty body |

Create/update request fields are `name`, `type`, `description`, `status`, `latitude`, and `longitude`. The backend rejects unknown JSON fields. Entity responses additionally contain `id`, `created_at`, and `updated_at`.

Typical error statuses are `400` for malformed JSON, invalid UUIDs, or invalid input; `404` for a valid UUID with no matching entity; `405` for unsupported methods; and `500` for unexpected server errors. Internal database details are not returned to API clients.

Example create request:

```sh
curl -i http://localhost:8080/api/entities \
  -H 'Content-Type: application/json' \
  -d '{"name":"Warehouse sensor","type":"iot_device","description":"North entrance","status":"active","latitude":-6.2088,"longitude":106.8456}'
```

## Validation

### Frontend

- Trims and requires a non-empty name.
- Accepts only `vehicle`, `iot_device`, `facility`, or `other` types.
- Accepts only `active`, `inactive`, or `maintenance` statuses.
- Requires finite latitude in `[-90, 90]` and longitude in `[-180, 180]`.
- Shows field errors and prevents invalid form submission. Backend errors are shown in the form and as notifications.

### Backend

- Independently validates create and update requests.
- Trims and requires the name; rejects unsupported type/status values.
- Rejects missing, non-finite, or out-of-range coordinates, malformed JSON, unknown input fields, oversized bodies, and invalid UUID path values.
- Performs validation before writing to SQLite. A malformed UUID returns `400`; a valid UUID for a missing entity returns `404`.

## Development and production considerations

- Start the backend and Vite in separate terminals for local development. The Vite proxy keeps `/api` browser requests same-origin; it is not part of the production build.
- In production, configure the hosting reverse proxy to serve the frontend and forward `/api/*` to the Go backend on the same origin. This repository does not contain deployment manifests or a configured reverse proxy.
- If separate frontend and backend origins are introduced later, add a strict backend CORS origin allowlist and preflight handling. Wildcard CORS is not enabled.
- The map uses OpenStreetMap tiles and keeps attribution visible. Any deployment must preserve that attribution and follow the tile provider's current usage policy.
- Keep the SQLite database on persistent storage and back it up according to the deployment environment. The MVP does not implement authentication or database replication.

## Known limitations

- No authentication or authorization; this is an unauthenticated take-home MVP.
- No production hosting configuration, reverse proxy, or cross-origin CORS allowlist is included.
- Browser end-to-end automation was not available during verification. Map interactions have unit/code-level checks and require the manual checks below before release.
- The default storage is a local SQLite file intended for this MVP's single-service deployment shape.

### Manual map verification

1. Start the backend and frontend using the commands above.
2. Click **Add Entity**, click a location on the map, and confirm the draft marker and coordinate inputs move to that point.
3. Complete the form and create the entity; confirm its saved marker and details appear.
4. Select the entity and click **Center** twice; confirm each click centers the map on its coordinates.
5. Edit the latitude and longitude, save, and confirm the details and marker move to the saved coordinates.
6. Delete the entity and confirm its marker disappears. Refresh the page and confirm the deletion persists.
