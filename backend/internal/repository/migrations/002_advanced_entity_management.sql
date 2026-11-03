CREATE TABLE attribute_definitions (
    id TEXT PRIMARY KEY NOT NULL,
    entity_type TEXT NOT NULL CHECK(entity_type IN ('vehicle','iot_device','facility','other')),
    key TEXT NOT NULL CHECK(length(key) BETWEEN 1 AND 64),
    label TEXT NOT NULL CHECK(length(trim(label)) BETWEEN 1 AND 100),
    value_type TEXT NOT NULL CHECK(value_type IN ('string','number','boolean')),
    required INTEGER NOT NULL CHECK(required IN (0,1)),
    min_number REAL,
    max_number REAL,
    max_length INTEGER CHECK(max_length IS NULL OR max_length BETWEEN 1 AND 2048),
    archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(entity_type,key),
    CHECK(min_number IS NULL OR max_number IS NULL OR min_number <= max_number)
);
CREATE INDEX idx_attribute_definitions_type ON attribute_definitions(entity_type,archived,key);

CREATE TABLE entity_attribute_values (
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    definition_id TEXT NOT NULL REFERENCES attribute_definitions(id) ON DELETE RESTRICT,
    text_value TEXT,
    number_value REAL,
    boolean_value INTEGER CHECK(boolean_value IS NULL OR boolean_value IN (0,1)),
    PRIMARY KEY(entity_id,definition_id),
    CHECK((text_value IS NOT NULL) + (number_value IS NOT NULL) + (boolean_value IS NOT NULL) = 1)
);
CREATE INDEX idx_entity_attribute_definition ON entity_attribute_values(definition_id,entity_id);

CREATE TABLE tags (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 64),
    normalized_name TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);
CREATE TABLE entity_tags (
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
    PRIMARY KEY(entity_id,tag_id)
);
CREATE INDEX idx_entity_tags_tag ON entity_tags(tag_id,entity_id);

CREATE TABLE categories (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
    normalized_name TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);
CREATE TABLE entity_categories (
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    PRIMARY KEY(entity_id,category_id)
);
CREATE INDEX idx_entity_categories_category ON entity_categories(category_id,entity_id);

CREATE TABLE activity_events (
    id TEXT PRIMARY KEY NOT NULL,
    entity_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK(event_type IN ('entity.created','entity.updated','entity.deleted','entity.attributes.updated','entity.tags.updated','entity.categories.updated')),
    occurred_at TEXT NOT NULL,
    actor_kind TEXT NOT NULL CHECK(actor_kind='system'),
    actor_id TEXT,
    payload_version INTEGER NOT NULL CHECK(payload_version=1),
    payload TEXT NOT NULL CHECK(length(CAST(payload AS BLOB)) <= 65536)
);
CREATE INDEX idx_activity_entity_time ON activity_events(entity_id,occurred_at DESC,id DESC);

CREATE TRIGGER activity_events_no_update BEFORE UPDATE ON activity_events BEGIN SELECT RAISE(ABORT,'activity events are append-only'); END;
CREATE TRIGGER activity_events_no_delete BEFORE DELETE ON activity_events BEGIN SELECT RAISE(ABORT,'activity events are append-only'); END;
