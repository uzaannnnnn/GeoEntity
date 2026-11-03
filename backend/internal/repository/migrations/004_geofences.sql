CREATE TABLE geofences (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 100),
    normalized_name TEXT NOT NULL UNIQUE CHECK(length(normalized_name) BETWEEN 1 AND 100),
    enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
    geometry_type TEXT NOT NULL CHECK(geometry_type IN ('Circle','Polygon')),
    geometry_json TEXT NOT NULL CHECK(length(CAST(geometry_json AS BLOB)) BETWEEN 1 AND 65536),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_geofences_enabled_created_at_id
    ON geofences(enabled,created_at,id);
