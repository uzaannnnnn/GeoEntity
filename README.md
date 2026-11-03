# GeoEntity

GeoEntity is a map-based application for managing entities with geographic coordinates. The React and TypeScript frontend uses Leaflet and calls a Go HTTP API; the API validates requests and stores data in SQLite.

The published application includes entity CRUD and search, custom attributes/tags/categories and activity history, circle and polygon geofences with membership/event history, administrator authentication, IoT device management and telemetry monitoring, and JSON/CSV data exchange.

## Architecture

~~~text
React + TypeScript + React Leaflet
        │ same-origin /api requests
        ▼
Vite proxy (development only) ──► Go net/http server
                                      │ authentication, CSRF, validation
                                      ▼
                               handler and domain services
                                      │
                                      ▼
                              SQLite repository + migrations
~~~

Vite's proxy is for local development. A production deployment must serve the frontend and route /api/* through a same-origin, TLS-terminating reverse proxy to the Go service. This repository does not include a production hosting or proxy configuration.

## Technology choices

| Area | Technology | Reason |
|---|---|---|
| Frontend | React 19 + TypeScript | Component-based UI with typed API and entity state. |
| Frontend tooling | Vite 6 | Small development-server setup and TypeScript-checked production build. |
| Map | Leaflet + React Leaflet | Interactive maps and markers integrated with React component lifecycles. |
| Map tiles | OpenStreetMap by default | No application API key is required; attribution remains visible. |
| Backend | Go net/http | Standard-library HTTP server and middleware without a web framework dependency. |
| Persistence API | Go database/sql | Standard SQL interface separates repository implementation from handlers. |
| SQLite driver | modernc.org/sqlite | SQLite through database/sql without a CGO/native build requirement. |
| Storage | SQLite | Simple, durable local storage suitable for this single-service application. |
| Frontend tests | Vitest | Runs the existing frontend unit and API tests. |
| Administrator password hashing | Argon2id | Memory-hard password hashing implemented in the authentication service. |

## Prerequisites

- Go 1.27.3 or later (see backend/go.mod).
- Node.js 20.x, 22.x, or 24+ and npm (see the frontend lockfile for exact package versions).
- Browser internet access for OpenStreetMap tiles and external fonts.

## Local development setup

The commands below use a temporary-development profile on loopback. It enables a non-Secure cookie for the local HTTP frontend only. Do not use GEOENTITY_DEV_COOKIE_INSECURE=true outside a loopback development environment.

### 1. Install project dependencies

~~~sh
cd backend
go mod download
cd ../frontend
npm ci
cp .env.example .env
~~~

### 2. Create the local administrator

From the repository root, run the CLI from backend/:

~~~sh
cd backend
GEOENTITY_DB_PATH=./data/geoentity.db \
GEOENTITY_PUBLIC_ORIGIN=http://localhost:5173 \
GEOENTITY_DEV_COOKIE_INSECURE=true \
go run ./cmd/geoentity admin provision
~~~

The CLI prompts twice for the administrator passphrase without echoing it. Use at least 15 Unicode code points and choose a unique passphrase. The command opens the configured database, applies pending embedded migrations, and provisions the single administrator. The password is not accepted as a command-line argument.

### 3. Start the backend

In a second terminal:

~~~sh
cd backend
GEOENTITY_ADDR=127.0.0.1:8080 \
GEOENTITY_DB_PATH=./data/geoentity.db \
GEOENTITY_PUBLIC_ORIGIN=http://localhost:5173 \
GEOENTITY_DEV_COOKIE_INSECURE=true \
go run ./cmd/api
~~~

The server creates the database parent directory and applies pending migrations on startup. The default bind is 127.0.0.1:8080; the default database path is ./data/geoentity.db, relative to the backend process working directory.

### 4. Start the frontend

In a third terminal:

~~~sh
cd frontend
npm test
npm run dev
~~~

Vite serves http://localhost:5173 and proxies /api to http://localhost:8080 by default. Open that address and sign in with the administrator passphrase created above. The UI checks the session on load and provides a sign-in form when there is no valid session.

### 5. Sign-in, sessions, and API requests

The browser uses GET /api/auth/session to check the session and obtain the CSRF token. POST /api/auth/login accepts the administrator passphrase and creates an HttpOnly session cookie; the UI keeps the returned CSRF token in memory. POST /api/auth/logout revokes the session and clears the cookie.

Protected API requests require the session cookie. State-changing requests also require the configured exact Origin and an X-CSRF-Token header. The frontend API client handles these details. GET /api/health is public. Device telemetry ingestion uses its separately provisioned device credential; it does not use the administrator browser session.

Session cookies are Secure by default. When serving over HTTPS, keep GEOENTITY_DEV_COOKIE_INSECURE unset or false and set GEOENTITY_PUBLIC_ORIGIN to the frontend's exact HTTPS origin. The insecure development setting is accepted only when GEOENTITY_ADDR is loopback.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| GEOENTITY_ADDR | 127.0.0.1:8080 | Go HTTP listen address. |
| GEOENTITY_DB_PATH | ./data/geoentity.db | SQLite database path relative to the backend working directory. |
| GEOENTITY_PUBLIC_ORIGIN | https://localhost:5173 | Exact frontend origin used for authentication and CSRF checks. |
| GEOENTITY_DEV_COOKIE_INSECURE | unset (false) | Disables Secure cookies only for local loopback development. |
| GEOENTITY_TRUSTED_PROXY_CIDRS | empty | Comma-separated trusted proxy CIDRs for forwarded client IP handling. |
| GEOENTITY_BACKUP_DIR | ./backups | Directory for operator-managed verified backups. |
| GEOENTITY_BACKUP_KEEP | 10 | Backup retention count for the CLI backup command. |
| VITE_API_BASE_URL | /api | Frontend API base URL. |
| VITE_PROXY_TARGET | http://localhost:8080 | Vite development proxy target. |
| VITE_MAP_TILE_URL | OpenStreetMap standard tile URL | Tile provider URL template. |
| VITE_MAP_ATTRIBUTION | OpenStreetMap attribution | Attribution displayed for the selected map tiles. |
| VITE_MAP_MAX_ZOOM | 19 | Maximum tile zoom level. |

Backend example variables are in backend/.env.example; Vite example variables are in frontend/.env.example. The Go server does not load dotenv files: set backend variables in its process environment. Do not put secrets in VITE_ variables because they are bundled into browser code.

## Database and migrations

SQLite is the persistent source of truth. The repository embeds versioned SQL migrations from backend/internal/repository/migrations/ and applies pending migrations during repository.Open at startup. Keep GEOENTITY_DB_PATH stable and point it at persistent storage when running outside local development. The database and backup directories are excluded from Git.

The application is designed for one API process per SQLite database. Use the CLI for verified backups and restores; keep backup files under operator control. Never test against a valuable database—use a temporary database path.

## Entity and map workflows

- **View:** the map displays entity markers from the viewport-aware map API. The list supports search and type/status filters. Select a marker or list row to open entity details.
- **Create:** choose Add Entity, select a map location or enter coordinates, fill in the form, and save. The draft marker can be repositioned; coordinates and entity fields are validated in the browser and again by the backend.
- **Update:** open an entity's details, choose Edit, change its fields or coordinates, and save. The marker and details are updated from the API response and reconciled with stored data.
- **Delete:** choose Delete from the entity details and confirm. The API removes the entity; the marker and list entry are removed after success.
- **Details:** the details panel shows the entity identity, type, status, description, coordinates, timestamps, and available related metadata/history sections.

The application also supports custom attribute definitions and values, tags/categories, entity activity history, circle/polygon geofence definitions and transition history, and import/export workflows. The Monitoring view displays device and sensor status/history but is read-only. Administrators provision and manage device credentials with the CLI; a credential is displayed when provisioned, rotated, or reassociated and cannot be recovered later.

## API overview

Core entity routes are under /api:

| Method | Path | Purpose |
|---|---|---|
| GET | /api/health | Public health check. |
| GET | /api/entities | List entities. |
| GET | /api/entities/{id} | Read entity details. |
| POST | /api/entities | Create an entity. |
| PUT | /api/entities/{id} | Update an entity. |
| DELETE | /api/entities/{id} | Delete an entity. |
| GET | /api/entities/map | Query map markers by viewport and filters. |
| GET | /api/entities/query | Search and paginate entities. |

Except for health, authentication bootstrap, and the separately authenticated device-ingestion route, API routes require administrator authentication. Protected mutating requests require a valid CSRF token and allowed origin. Do not use unauthenticated curl examples for protected routes; follow the browser sign-in flow or implement the session and CSRF protocol explicitly.

## Validation

The frontend validates required names, supported entity types/statuses, and finite latitude/longitude bounds before sending a request. The Go backend independently validates these fields, JSON shape and size, unknown fields, IDs, query parameters, and feature-specific constraints before database writes. Server validation is authoritative.

## Tests and production build

Run from backend/:

~~~sh
go test ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
~~~

Run from frontend/:

~~~sh
npm test
npm run build
~~~

The production build creates frontend/dist/. The current build may emit a non-blocking advisory when the minified JavaScript chunk exceeds 500 kB; the last recorded build produced a 604.37 kB chunk. Recheck this when dependencies or bundling change.

## Manual browser smoke check

With the local services running, use a synthetic entity and verify:

1. Sign in, open the map, and confirm existing markers appear at their stored coordinates.
2. Start **Add Entity**, choose a location on the map, save the form, and confirm the marker and details appear.
3. Select the marker and confirm its details identify the same entity and coordinates.
4. Edit the entity and move its coordinates; confirm the marker and details move, then reload and confirm the saved state remains.
5. Try a missing name and out-of-range coordinates; confirm the form/backend rejects them and no invalid entity is stored.
6. Delete the entity and confirm its marker/details disappear and a refresh does not restore it.
7. Sign out and confirm protected data requires signing in again. Check the browser console and network panel for unexpected errors or failed requests.

This checklist is a suggested manual verification procedure, not a record that these browser steps have passed. Capture screenshots when performing a release QA run.

## Production prerequisites and limitations

- Terminate TLS at a trusted reverse proxy, serve the frontend and /api/* on the same HTTPS origin, and set GEOENTITY_PUBLIC_ORIGIN to that exact origin.
- Keep Secure cookies enabled. Bind the Go service to loopback behind the proxy where possible; a non-loopback bind requires explicit GEOENTITY_TRUSTED_PROXY_CIDRS and a TLS-terminating proxy. The service does not include a production reverse-proxy, deployment, or hosting configuration.
- SQLite permits a single writer; this application supports one API process per database. Login and mutation rate-limit state is process-local and does not enforce limits across multiple hosts.
- Telemetry observations are append-only and have no automatic retention or pruning policy, so database and backup size grows over time.
- Browser end-to-end automation is not included in the repository. The unit/integration suites do not replace a real browser check of login, map markers, entity CRUD, loading/error states, and logout before release.
- OpenStreetMap's standard tile service is best-effort, has no SLA, and must retain visible attribution. Follow the provider's current usage policy; use an appropriate commercial or self-hosted provider for public/high-volume production.
- No production deployment or external recovery exercise is claimed by this repository documentation.
