CREATE TABLE entity_geofence_memberships (
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK(state IN ('inside','outside','inactive')),
    evaluated_latitude REAL NOT NULL CHECK(evaluated_latitude BETWEEN -90 AND 90),
    evaluated_longitude REAL NOT NULL CHECK(evaluated_longitude BETWEEN -180 AND 180),
    evaluated_at TEXT NOT NULL,
    definition_updated_at TEXT NOT NULL,
    PRIMARY KEY(entity_id,geofence_id)
);

CREATE INDEX idx_entity_geofence_memberships_geofence_state_entity
    ON entity_geofence_memberships(geofence_id,state,entity_id);

CREATE TABLE geofence_events (
    id TEXT PRIMARY KEY NOT NULL,
    entity_id TEXT NOT NULL,
    entity_name TEXT NOT NULL,
    geofence_id TEXT NOT NULL,
    geofence_name TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK(event_type IN ('enter','exit')),
    latitude REAL NOT NULL CHECK(latitude BETWEEN -90 AND 90),
    longitude REAL NOT NULL CHECK(longitude BETWEEN -180 AND 180),
    occurred_at TEXT NOT NULL,
    source TEXT NOT NULL CHECK(source='coordinate_update')
);

CREATE INDEX idx_geofence_events_geofence_time_id
    ON geofence_events(geofence_id,occurred_at DESC,id DESC);
CREATE INDEX idx_geofence_events_entity_time_id
    ON geofence_events(entity_id,occurred_at DESC,id DESC);

CREATE TRIGGER geofence_events_no_update
BEFORE UPDATE ON geofence_events
BEGIN
    SELECT RAISE(ABORT,'geofence events are append-only');
END;

CREATE TRIGGER geofence_events_no_delete
BEFORE DELETE ON geofence_events
BEGIN
    SELECT RAISE(ABORT,'geofence events are append-only');
END;
