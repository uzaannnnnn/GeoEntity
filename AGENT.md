# Agentic AI Development Record

Agentic AI assisted GeoEntity development under human direction. The existing phase records in MASTER_PROMPT.md, HANDOFF.md, the V2 phase plans, and QA reports remain the evidence trail. Code was not represented as manually written entirely by a human.

## Agents and responsibilities

- **Codex — GPT-6 Luna:** requirement analysis, MVP_CONTRACT.md, Go backend and SQLite implementation, REST API and validation, integration, backend fixes, documentation, regression verification, and repository preparation.
- **Antigravity — Gemini Flash 3.8:** React, TypeScript, Vite, Leaflet frontend, map interactions, frontend validation and UI behavior, frontend tests/build, and the independent PHASE 4 quality audit recorded in the handoff.
- **Human project owner:** selected when phases began, authorized handoffs, confirmed the repository root, reviewed results, and retained control over phase transitions.

The recorded MVP workflow was sequential:

1. **PHASE 0 — Codex:** analyzed the take-home scope and wrote MVP_CONTRACT.md and HANDOFF.md.
2. **PHASE 1 — Codex:** implemented the Go API, SQLite schema/persistence, validation, error responses, configuration, and backend integration tests.
3. **PHASE 2 — Antigravity:** implemented the Vite/React/TypeScript UI, Leaflet map, forms, API calls, validation, UI states, frontend unit tests, and production build.
4. **PHASE 3 — Codex:** checked API/environment compatibility, exercised CRUD, validation, and persistence through the Vite proxy, selected same-origin production routing, and recorded UI issues that could not be browser-tested.
5. **PHASE 4 — Antigravity:** performed the requested independent QA and returned **PASS WITH ISSUES**, including findings about the Center action and marker reconciliation.
6. **PHASE 5 — Codex:** addressed those findings, added regression tests, updated documentation, reran verification, and prepared the MVP submission.

Later V2.1–V2.8 work expanded the application. The phase-specific handoffs and reports describe those later changes and their verification. Each phase began only after human authorization. The agents did not autonomously start a later phase or assign work to parallel workers. PHASE 4 was read-only; fixes were made only in the separately authorized PHASE 5.

## Current application and security architecture

The current published application has a single CLI-provisioned administrator. The command go run ./cmd/geoentity admin provision prompts twice for a passphrase; passwords are normalized and validated, then stored as Argon2id hashes. The browser receives a random opaque session cookie marked HttpOnly (Secure by default); the database stores a hash of the session token. Sessions have idle and absolute expiry. Protected mutation requests require the allowed Origin and a CSRF token in X-CSRF-Token. The frontend obtains session/CSRF state through the auth API. Device telemetry uses a separate device credential; raw device credentials are shown at provisioning, rotation, or reassociation and are not recoverable.

The server defaults to Secure cookies. GEOENTITY_DEV_COOKIE_INSECURE=true is a loopback-only development option. A non-loopback server bind requires an explicitly configured trusted-proxy CIDR and TLS-terminating proxy. GEOENTITY_PUBLIC_ORIGIN must match the browser-facing origin. See the current README.md and backend/.env.example for setup details.

## Verification approach and limits

The project records Go unit/integration tests, race tests, go vet, builds, frontend Vitest tests, TypeScript/Vite builds, and temporary-database HTTP CRUD checks. Backend tests use isolated temporary database paths; do not point tests at a persistent user database. Browser-driven behavior must be recorded only when a supported browser and automation/manual QA are actually available. Source review and unit tests are not browser evidence.

The V2.8A QA report records its test scope and gaps. Browser end-to-end verification was not completed in that run. The current submission-readiness audit also could not perform browser QA because a supported browser and automation mechanism were unavailable. The current repository does not include a browser end-to-end suite; the manual flow in README.md is a release check, not evidence that it has passed.

## Known operating limitations

- SQLite is a single-writer store and the supported service model is one API process per database.
- Login and mutation rate limits are process-local and do not coordinate across multiple hosts.
- Telemetry is append-only with no automatic retention policy; database and backup size grows over time.
- Production hosting, TLS reverse-proxy configuration, and external recovery operations are not supplied or claimed as deployed.
- The V2.8A frontend build emitted a non-blocking large-chunk advisory (604.37 kB minified JavaScript); recheck after dependency or bundler changes.
- External browser rendering depends on network access to the configured map tile provider and fonts.

AI-assisted output still requires human review for correctness, security, accessibility, deployment configuration, and fit to the assignment. Test coverage does not prove the absence of defects; environment-specific results may differ on another machine.
