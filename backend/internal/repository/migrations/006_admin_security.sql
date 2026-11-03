CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    password_verifier TEXT NOT NULL,
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK(consecutive_failures >= 0),
    next_login_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE web_sessions (
    token_hash TEXT PRIMARY KEY NOT NULL,
    csrf_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    absolute_expires_at TEXT NOT NULL
);
CREATE INDEX idx_web_sessions_expiry ON web_sessions(absolute_expires_at,last_seen_at);

CREATE TABLE admin_audit_events (
    id TEXT PRIMARY KEY NOT NULL,
    occurred_at TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_label TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    payload_version INTEGER NOT NULL DEFAULT 1 CHECK(payload_version = 1),
    details TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_admin_audit_events_time_id ON admin_audit_events(occurred_at DESC,id DESC);
CREATE INDEX idx_admin_audit_events_actor_time ON admin_audit_events(actor_id,occurred_at DESC);
CREATE TRIGGER admin_audit_events_no_update BEFORE UPDATE ON admin_audit_events BEGIN SELECT RAISE(ABORT,'admin audit events are append-only'); END;
CREATE TRIGGER admin_audit_events_no_delete BEFORE DELETE ON admin_audit_events BEGIN SELECT RAISE(ABORT,'admin audit events are append-only'); END;
