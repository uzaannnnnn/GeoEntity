CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    entity_id TEXT REFERENCES entities(id) ON DELETE RESTRICT,
    label TEXT NOT NULL CHECK(length(label) BETWEEN 1 AND 120),
    status TEXT NOT NULL CHECK(status IN ('active','revoked','detached')),
    expected_interval_seconds INTEGER CHECK(expected_interval_seconds IS NULL OR expected_interval_seconds BETWEEN 1 AND 31536000),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_seen_at TEXT,
    revoked_at TEXT,
    CHECK(status!='active' OR entity_id IS NOT NULL),
    CHECK(status!='revoked' OR revoked_at IS NOT NULL)
);
CREATE INDEX idx_devices_entity_status_id ON devices(entity_id,status,id);

CREATE TABLE device_credentials (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    secret_hash TEXT NOT NULL CHECK(length(secret_hash)=64 AND secret_hash NOT GLOB '*[^0-9a-f]*'),
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at TEXT
);
CREATE UNIQUE INDEX idx_device_credentials_one_active ON device_credentials(device_id) WHERE revoked_at IS NULL;

CREATE TABLE device_sensors (
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    sensor_id TEXT NOT NULL CHECK(length(sensor_id) BETWEEN 1 AND 64 AND sensor_id GLOB '[a-z]*' AND sensor_id NOT GLOB '*[^a-z0-9_]*'),
    unit TEXT NOT NULL CHECK(unit IN ('Cel','%','V','A','m','Pa','ppm','dBm','1')),
    created_at TEXT NOT NULL,
    PRIMARY KEY(device_id,sensor_id)
);
CREATE TRIGGER device_sensors_no_update BEFORE UPDATE ON device_sensors BEGIN SELECT RAISE(ABORT,'device sensor units are immutable'); END;
CREATE TRIGGER device_sensors_no_delete BEFORE DELETE ON device_sensors BEGIN SELECT RAISE(ABORT,'device sensor records are append-only'); END;

CREATE TABLE sensor_observations (
    id TEXT PRIMARY KEY,
    entity_id TEXT NOT NULL,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    credential_id TEXT NOT NULL REFERENCES device_credentials(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL CHECK(length(idempotency_key) BETWEEN 1 AND 128),
    sensor_id TEXT NOT NULL CHECK(length(sensor_id) BETWEEN 1 AND 64),
    value REAL NOT NULL CHECK(typeof(value) IN ('real','integer') AND value BETWEEN -1.0e100 AND 1.0e100),
    unit TEXT NOT NULL CHECK(unit IN ('Cel','%','V','A','m','Pa','ppm','dBm','1')),
    observed_at TEXT NOT NULL CHECK(length(observed_at)>0),
    ingested_at TEXT NOT NULL CHECK(length(ingested_at)>0),
    payload_sha256 TEXT NOT NULL CHECK(length(payload_sha256)=64 AND payload_sha256 NOT GLOB '*[^0-9a-f]*'),
    metadata TEXT CHECK(metadata IS NULL OR (length(metadata)<=1024 AND json_valid(metadata) AND json_type(metadata)='object')),
    UNIQUE(device_id,idempotency_key),
    FOREIGN KEY(device_id,sensor_id) REFERENCES device_sensors(device_id,sensor_id) ON DELETE RESTRICT
);
CREATE INDEX idx_sensor_observations_entity_sensor_time ON sensor_observations(entity_id,sensor_id,observed_at DESC,id DESC);
CREATE INDEX idx_sensor_observations_entity_time ON sensor_observations(entity_id,observed_at DESC,id DESC);
CREATE INDEX idx_sensor_observations_device_time ON sensor_observations(device_id,observed_at DESC,id DESC);
CREATE TRIGGER sensor_observations_validate_insert BEFORE INSERT ON sensor_observations
WHEN NOT EXISTS (
    SELECT 1 FROM devices d
    JOIN entities e ON e.id=d.entity_id
    JOIN device_credentials c ON c.id=NEW.credential_id AND c.device_id=d.id
    JOIN device_sensors s ON s.device_id=d.id AND s.sensor_id=NEW.sensor_id AND s.unit=NEW.unit
    WHERE d.id=NEW.device_id AND d.entity_id=NEW.entity_id AND d.status='active' AND e.status='active' AND c.revoked_at IS NULL
)
BEGIN SELECT RAISE(ABORT,'observation scope or sensor unit is invalid'); END;
CREATE TRIGGER sensor_observations_no_update BEFORE UPDATE ON sensor_observations BEGIN SELECT RAISE(ABORT,'sensor observations are append-only'); END;
CREATE TRIGGER sensor_observations_no_delete BEFORE DELETE ON sensor_observations BEGIN SELECT RAISE(ABORT,'sensor observations are append-only'); END;
