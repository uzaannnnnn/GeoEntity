# MASTER PROJECT INSTRUCTION
## GeoEntity Management System — Software Developer Take-Home Test

## 1. PROJECT CONTEXT

You are participating in the development of a full-stack web application called **GeoEntity Management System**.

This project is based on a Software Developer Take-Home Test.

The application manages generic real-world entities that contain geographic location information.

Examples of entities:
- Vehicles
- IoT devices
- Facility locations
- Other real-world objects

Every entity must have:
- Unique identity
- Basic attributes
- Status
- Geographic coordinates (latitude and longitude)

### Mandatory requirements from the assignment

1. Frontend must use React + TypeScript.
2. Backend must use Go.
3. Data must be stored and retrieved through the backend.
4. The application must display entities on an interactive map.
5. Users must be able to create entities.
6. Users must be able to update entities.
7. Users must be able to delete entities.
8. Users must be able to view entity details on the map.
9. Both frontend and backend must implement input/data validation.
10. Documentation must explain:
   - How to run the application.
   - Why the selected libraries were chosen.
   - How Agentic AI was used during development.

If Agentic AI is used, the repository must contain `AGENT.md`.

These requirements are mandatory. Do not sacrifice them to implement unnecessary features.

---

## 2. DEVELOPMENT STRATEGY

Use a pragmatic MVP-first development approach.

The goal is to deliver a complete, working, maintainable application within the two-day deadline.

Prioritize:

1. Functional correctness.
2. Backend/frontend integration.
3. Data persistence.
4. Input validation.
5. Map interaction.
6. User experience.
7. Documentation.

Avoid overengineering.

Do not implement features that are outside the MVP unless all mandatory requirements have been completed.

### Explicitly excluded from the MVP

- Authentication and authorization.
- Multi-role user management.
- Real-time GPS tracking.
- WebSocket.
- Geofencing.
- Vehicle route optimization.
- Analytics dashboard.
- Complex GIS analysis.
- Mobile application.
- Microservices architecture.

---

## 3. TECHNOLOGY STACK

Use the following stack unless an existing repository constraint makes it impossible. Report any unavoidable deviation.

### Frontend
- React
- TypeScript
- Vite
- Leaflet
- React Leaflet
- OpenStreetMap
- CSS or Tailwind CSS

### Backend
- Go
- Standard `net/http` package
- `database/sql`
- SQLite

### Architecture
- REST API
- JSON communication
- Monolithic application with separated frontend/backend directories.

Avoid introducing unnecessary dependencies.

For SQLite, select a compatible and maintained Go database driver. Prefer a solution that does not require unnecessary native build configuration.

Use environment variables for configurable settings where appropriate.

---

## 4. ENTITY DATA MODEL

Implement a generic entity model.

### Entity

| Field | Type | Required |
|---|---|---|
| id | UUID | Yes |
| name | String | Yes |
| type | String | Yes |
| description | String | No |
| status | String | Yes |
| latitude | Float64 | Yes |
| longitude | Float64 | Yes |
| created_at | Timestamp | Yes |
| updated_at | Timestamp | Yes |

### Allowed entity types

- `vehicle`
- `iot_device`
- `facility`
- `other`

### Allowed statuses

- `active`
- `inactive`
- `maintenance`

These values are MVP design decisions, not additional requirements imposed by the assignment.

### Database rules

- Use UUID as the entity identifier.
- Latitude must be within -90 and 90.
- Longitude must be within -180 and 180.
- Store timestamps consistently in UTC.
- Persist data in SQLite.
- Do not use frontend state or mock data as the primary data source.

---

## 5. REST API CONTRACT

Implement these endpoints:

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/health` | Backend health check |
| GET | `/api/entities` | Retrieve all entities |
| GET | `/api/entities/{id}` | Retrieve one entity |
| POST | `/api/entities` | Create entity |
| PUT | `/api/entities/{id}` | Update entity |
| DELETE | `/api/entities/{id}` | Delete entity |

### Response format

Successful entity responses:

`{"data": {...}}`

Successful list response:

`{"data": [...]}`

Error response:

`{"error":{"code":"VALIDATION_ERROR","message":"Invalid input","field":"latitude"}}`

### Expected HTTP status codes

- 200: Successful GET or PUT.
- 201: Successful POST.
- 204: Successful DELETE.
- 400: Invalid request or validation error.
- 404: Entity not found.
- 405: Method not allowed.
- 500: Internal server error.

Maintain consistent response structures.

Never expose internal database errors, stack traces, or sensitive implementation details in public API responses.

Document the final API contract in `MVP_CONTRACT.md`.

---

## 6. VALIDATION REQUIREMENTS

Validation must exist independently on both frontend and backend.

### Frontend validation

- Name cannot be empty.
- Type must be a supported value.
- Status must be a supported value.
- Latitude must be between -90 and 90.
- Longitude must be between -180 and 180.
- Show understandable validation messages.
- Do not submit invalid forms.
- Handle API errors gracefully.

### Backend validation

- Validate every incoming create/update request.
- Trim and validate required strings.
- Reject unsupported type and status values.
- Reject out-of-range coordinates.
- Reject NaN and infinite coordinate values.
- Reject malformed JSON.
- Handle missing or invalid entity IDs.
- Do not write invalid data into the database.

Frontend validation is not a substitute for backend validation.

Backend validation is the final source of truth.

---

## 7. USER INTERFACE REQUIREMENTS

Use a clean, professional, map-centric dashboard.

The map should be the primary working area.

### Main layout

Header:
- Application name: GeoEntity.
- Total entity count.
- Add Entity button.

Main area:
- Interactive map.
- Entity markers.
- Entity side panel.

Side panel:
- Entity list.
- Selected entity details.
- Create/edit form when required.

### Required user workflows

#### A. View entities
1. Open the application.
2. Frontend retrieves entity data from the backend.
3. Display all entities as map markers.
4. Show an appropriate empty state when no data exists.

#### B. View entity details
1. User clicks a marker.
2. Display its entity information in a side panel or popup.
3. Display name, type, status, description, and coordinates.

#### C. Create entity
1. User clicks Add Entity.
2. Enter location selection mode.
3. User clicks on the map to select coordinates.
4. Display the create form with selected coordinates.
5. Allow coordinates to be edited manually.
6. Validate input.
7. Submit to the backend.
8. Refresh entity data and display the new marker.

#### D. Update entity
1. Select an entity.
2. Open its edit form.
3. Populate the existing values.
4. Allow attribute and coordinate changes.
5. Submit the update to the backend.
6. Refresh the marker to reflect updated coordinates.

#### E. Delete entity
1. Select an entity.
2. Click Delete.
3. Display a confirmation dialog.
4. Submit deletion to the backend.
5. Remove the marker after successful deletion.

### UI quality

- Responsive layout.
- Clear visual hierarchy.
- Accessible form labels.
- Clear loading states.
- Clear empty states.
- Clear error states.
- Consistent spacing and typography.
- Avoid excessive animations.
- Avoid unnecessary UI components.

Use a reasonable default map center and automatically fit the map to available entity markers when appropriate.

OpenStreetMap attribution must remain visible.

Do not require an API key for the basic map implementation.

---

## 8. PROJECT STRUCTURE

Use a simple modular structure.

Suggested layout:

```text
geoentity/
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go
│   ├── internal/
│   │   ├── entity/
│   │   ├── handler/
│   │   ├── service/
│   │   └── repository/
│   ├── migrations/
│   ├── go.mod
│   └── .env.example
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── features/
│   │   ├── hooks/
│   │   ├── types/
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── package.json
│   └── .env.example
│
├── docs/
│   └── ...
│
├── MVP_CONTRACT.md
├── HANDOFF.md
├── AGENT.md
├── README.md
└── .gitignore
```

This is a suggested structure, not a reason to refactor an already-working project unnecessarily.

---

## 9. SEQUENTIAL AGENT WORKFLOW

IMPORTANT:

This project uses TWO AI agents operated manually and sequentially.

They are NOT autonomous parallel agents.

Never assume another agent is working simultaneously.

Never spawn subagents, background workers, or parallel coding agents.

Never assign work to another agent automatically.

The human user controls every handoff.

The repository is the shared source of truth.

### Agent 1: CODEX — GPT-6 LUNA

Primary responsibilities:
- Requirement analysis.
- Technical planning.
- Backend development.
- SQLite implementation.
- REST API.
- Backend validation.
- Full-stack integration.
- Debugging.
- Testing.
- Final documentation.
- Final verification.

Reasoning configuration:
LOW for routine implementation.
MEDIUM when architectural judgment or complex debugging requires it.
HIGH only for genuinely difficult problems.

### Agent 2: ANTIGRAVITY — GEMINI FLASH 3.8

Primary responsibilities:
- Frontend development.
- React + TypeScript.
- Leaflet integration.
- UI/UX implementation.
- Frontend validation.
- Frontend quality review.
- Independent final QA audit.

Reasoning configuration:
HIGH as configured by the user.

During final QA, operate in READ-ONLY mode.

Do not modify backend files.

Do not alter source code during the independent audit.

---

## 10. EXECUTION PHASES

Only execute the phase explicitly assigned by the user.

Do not automatically move to the next phase.

### PHASE 0 — CODEX: REQUIREMENT ANALYSIS

Responsibilities:
1. Inspect the repository.
2. Check existing files and Git status.
3. Review the assignment requirements.
4. Define the final MVP scope.
5. Establish the data model.
6. Establish API request/response contracts.
7. Define frontend/backend validation rules.
8. Create `MVP_CONTRACT.md`.
9. Create `HANDOFF.md`.

Deliverable:
- MVP_CONTRACT.md
- HANDOFF.md
- A concise implementation plan.

Do not begin unnecessary frontend implementation.

### PHASE 1 — CODEX: BACKEND DEVELOPMENT

Responsibilities:
1. Initialize Go backend.
2. Configure SQLite.
3. Implement entity schema.
4. Implement repository layer.
5. Implement REST endpoints.
6. Implement CRUD operations.
7. Implement backend validation.
8. Implement consistent error handling.
9. Add backend tests.
10. Verify data persistence.

Deliverable:
- Working Go backend.
- SQLite persistence.
- Tested CRUD endpoints.
- Backend handoff documentation.

### PHASE 2 — ANTIGRAVITY: FRONTEND DEVELOPMENT

Responsibilities:
1. Inspect the existing project.
2. Read `MVP_CONTRACT.md`.
3. Read `HANDOFF.md`.
4. Understand the API contract.
5. Initialize React + TypeScript + Vite if necessary.
6. Integrate Leaflet.
7. Implement map markers.
8. Implement entity list.
9. Implement entity details.
10. Implement create form.
11. Implement update form.
12. Implement delete confirmation.
13. Implement frontend validation.
14. Implement loading, empty, and error states.
15. Verify TypeScript and production build.

STRICT BOUNDARIES:
- Do not modify backend logic.
- Do not redefine the API contract independently.
- Do not substitute API calls with mock data.
- Do not modify database schema.
- Do not introduce unnecessary dependencies.

Deliverable:
- Complete frontend implementation.
- Frontend verification report.
- Updated HANDOFF.md.

### PHASE 3 — CODEX: FULL-STACK INTEGRATION

Responsibilities:
1. Inspect the frontend implementation.
2. Inspect the backend implementation.
3. Verify API contract compatibility.
4. Configure frontend API base URL.
5. Resolve CORS configuration.
6. Test real API communication.
7. Test create/update/delete workflows.
8. Verify map marker synchronization.
9. Verify data persistence after refresh.
10. Fix integration issues.
11. Run frontend and backend tests.

Deliverable:
- Working integrated application.
- Integration test report.
- Updated HANDOFF.md.

### PHASE 4 — ANTIGRAVITY: INDEPENDENT QA AUDIT

Operate in READ-ONLY mode.

Inspect the actual source code, not just documentation.

Verify:

A. Assignment compliance
- React + TypeScript.
- Go backend.
- Backend persistence.
- Map visualization.
- Create entity.
- Update entity.
- Delete entity.
- View entity details.
- Frontend validation.
- Backend validation.

B. Backend
- API correctness.
- Database persistence.
- Coordinate validation.
- Error handling.
- HTTP status codes.

C. Frontend
- Marker correctness.
- CRUD behavior.
- Form validation.
- Details consistency.
- Loading/error/empty states.
- Coordinate updates.

D. Integration
- API contract consistency.
- Data survives page refresh.
- Markers reflect backend data.
- No unintended mock-data dependency.

Report findings using:

- P0: Critical, mandatory fix.
- P1: Major, should be fixed.
- P2: Minor, optional improvement.

For every finding include:
- File path.
- Exact problem.
- Impact.
- Reproduction steps or evidence.
- Recommended fix.

Do not modify files.

Do not report hypothetical problems as confirmed bugs.

Deliverable:
- Independent QA report.
- Prioritized findings.
- Updated HANDOFF.md.

### PHASE 5 — CODEX: FINAL FIX AND SUBMISSION

Responsibilities:
1. Review the QA report.
2. Verify each reported problem.
3. Fix P0 issues.
4. Fix P1 issues when feasible.
5. Perform final regression tests.
6. Verify frontend production build.
7. Verify backend tests.
8. Verify application startup from a clean environment.
9. Finalize README.md.
10. Finalize AGENT.md.
11. Update HANDOFF.md.
12. Check repository cleanliness and completeness.

Do not introduce new features at this stage.

Deliverable:
- Submission-ready repository.
- Final verification report.
- Documentation.

Do not automatically push to a remote repository or send emails without explicit user authorization.

---

## 11. DOCUMENTATION REQUIREMENTS

### README.md

Must explain:

1. Project overview.
2. Features.
3. Technologies used.
4. Reasons for library selection.
5. Project architecture.
6. Prerequisites.
7. Installation instructions.
8. How to run backend.
9. How to run frontend.
10. Environment configuration.
11. Database information.
12. REST API endpoints.
13. Validation rules.
14. Known limitations.

Instructions must be reproducible.

Do not invent commands or claim a feature works without verification.

### AGENT.md

Must transparently explain:

1. That Agentic AI was used.
2. Codex and its responsibilities.
3. Antigravity and its responsibilities.
4. Sequential workflow.
5. Human-directed handoffs.
6. How implementation was verified.
7. Areas reviewed or modified by the agents.

Never claim that all code was manually written by a human.

### HANDOFF.md

Record:
- Current project phase.
- Completed tasks.
- Changed files.
- Decisions made.
- Tests performed and results.
- Known issues.
- Next agent's responsibilities.

Keep this document concise and current.

---

## 12. ENGINEERING RULES

Follow these rules throughout development.

1. Inspect before modifying.
2. Read existing project conventions first.
3. Never overwrite unrelated files.
4. Never rewrite working modules without a concrete reason.
5. Keep dependencies minimal.
6. Use clear names and modular code.
7. Handle errors explicitly.
8. Never silently ignore failed API requests.
9. Never use hardcoded mock data as the actual persistence mechanism.
10. Do not expose sensitive configuration.
11. Do not claim tests passed without running them.
12. Do not mark an unfinished feature as complete.
13. Preserve working functionality when making fixes.
14. Prefer small, targeted changes.
15. Document deviations and incomplete features.

Before finishing a phase:
- Inspect Git diff.
- Run the relevant checks.
- Update HANDOFF.md.
- Report changed files.
- Report actual test outcomes.
- State remaining problems.

Be efficient with context and tokens.

Do not repeatedly print entire source files.

Do not repeatedly summarize the entire repository.

Read only the relevant files for the current task.

Do not produce unnecessary architectural essays.

---

## 13. DEFINITION OF DONE

The project is complete only when:

- [ ] Frontend uses React + TypeScript.
- [ ] Backend uses Go.
- [ ] Data persists through SQLite.
- [ ] Entities are displayed on the map.
- [ ] Entities can be created.
- [ ] Entities can be updated.
- [ ] Entities can be deleted.
- [ ] Entity details can be viewed.
- [ ] Frontend validation works.
- [ ] Backend validation works.
- [ ] Coordinates are correctly validated.
- [ ] Markers reflect saved coordinates.
- [ ] Data survives browser refresh.
- [ ] Frontend builds successfully.
- [ ] Backend tests pass.
- [ ] README.md is complete.
- [ ] AGENT.md is present.
- [ ] Known limitations are documented.

A feature is complete only after implementation and verification.

---

## 14. RESPONSE FORMAT

At the end of every phase, provide a concise report:

### Phase Status
Completed / Partially Completed / Blocked

### Work Performed
Brief summary.

### Files Changed
List relevant paths.

### Verification
Commands and actual results.

### Remaining Issues
Only confirmed or clearly identified issues.

### Handoff
Specify exactly which agent and phase should run next.

Do not start the next phase automatically.

Wait for the human user's explicit instruction.

---

# END OF MASTER PROJECT INSTRUCTION