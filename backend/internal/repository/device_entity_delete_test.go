package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
)

func openDeviceDeleteTestStore(t *testing.T) (*SQLite, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "entity-delete.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

func seedDeleteDevice(t *testing.T, db *sql.DB, entityID, deviceID, state string, withObservation bool) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO devices(id,entity_id,label,status,created_at,updated_at) VALUES(?,?,?,'active',?,?)`, deviceID, entityID, deviceID, now, now); err != nil {
		t.Fatal(err)
	}
	credentialID := "credential-" + deviceID
	if _, err := db.Exec(`INSERT INTO device_credentials(id,device_id,secret_hash,created_at) VALUES(?,?,?,?)`, credentialID, deviceID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now); err != nil {
		t.Fatal(err)
	}
	if withObservation {
		if _, err := db.Exec(`INSERT INTO device_sensors(device_id,sensor_id,unit,created_at) VALUES(?,'temperature','Cel',?)`, deviceID, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO sensor_observations(id,entity_id,device_id,credential_id,idempotency_key,sensor_id,value,unit,observed_at,ingested_at,payload_sha256) VALUES(?,?,?,?,?,'temperature',21.5,'Cel',?,?,?)`, "observation-"+deviceID, entityID, deviceID, credentialID, "key-"+deviceID, now, now, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err != nil {
			t.Fatal(err)
		}
	}
	switch state {
	case "active":
		return
	case "revoked":
		if _, err := db.Exec(`UPDATE device_credentials SET revoked_at=? WHERE device_id=?`, now, deviceID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE devices SET status='revoked',revoked_at=? WHERE id=?`, now, deviceID); err != nil {
			t.Fatal(err)
		}
	case "detached-associated":
		if _, err := db.Exec(`UPDATE devices SET status='detached' WHERE id=?`, deviceID); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown fixture state %q", state)
	}
}

func createDeleteTestEntity(t *testing.T, store *SQLite, id string) {
	t.Helper()
	now := time.Now().UTC()
	if err := store.Create(context.Background(), entity.Entity{ID: id, Name: "Delete target", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteEntityDetachesAllDeviceStatesAndPreservesHistory(t *testing.T) {
	ctx := context.Background()
	store, _ := openDeviceDeleteTestStore(t)
	db := store.DB()
	entityID := "10000000-0000-4000-8000-000000000201"
	createDeleteTestEntity(t, store, entityID)
	devices := []struct{ id, state string }{
		{"device-active", "active"},
		{"device-revoked", "revoked"},
		{"device-detached-associated", "detached-associated"},
	}
	for _, d := range devices {
		seedDeleteDevice(t, db, entityID, d.id, d.state, d.id == "device-active" || d.id == "device-revoked")
	}
	var originalRevokedAt string
	if err := db.QueryRow(`SELECT revoked_at FROM devices WHERE id='device-revoked'`).Scan(&originalRevokedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO devices(id,entity_id,label,status,created_at,updated_at,revoked_at) VALUES('already-detached',NULL,'already detached','detached',?,?,?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, entityID); err != nil {
		t.Fatalf("delete entity with mixed device states: %v", err)
	}
	var entityCount int
	if err := db.QueryRow(`SELECT count(*) FROM entities WHERE id=?`, entityID).Scan(&entityCount); err != nil || entityCount != 0 {
		t.Fatalf("entity count=%d err=%v", entityCount, err)
	}
	for _, d := range devices {
		var associated sql.NullString
		var status string
		var revokedAt sql.NullString
		if err := db.QueryRow(`SELECT entity_id,status,revoked_at FROM devices WHERE id=?`, d.id).Scan(&associated, &status, &revokedAt); err != nil {
			t.Fatal(err)
		}
		if associated.Valid || status != "detached" {
			t.Fatalf("device %s entity=%v status=%s", d.id, associated, status)
		}
		if d.id == "device-revoked" && (!revokedAt.Valid || revokedAt.String != originalRevokedAt) {
			t.Fatalf("revoked device timestamp changed from %q to %v", originalRevokedAt, revokedAt)
		}
		var activeCredentials int
		if err := db.QueryRow(`SELECT count(*) FROM device_credentials WHERE device_id=? AND revoked_at IS NULL`, d.id).Scan(&activeCredentials); err != nil || activeCredentials != 0 {
			t.Fatalf("device %s active credentials=%d err=%v", d.id, activeCredentials, err)
		}
	}
	var observationCount int
	var observedValue float64
	if err := db.QueryRow(`SELECT count(*),max(value) FROM sensor_observations WHERE entity_id=?`, entityID).Scan(&observationCount, &observedValue); err != nil || observationCount != 2 || observedValue != 21.5 {
		t.Fatalf("observations count=%d value=%v err=%v", observationCount, observedValue, err)
	}
	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM admin_audit_events WHERE action='device.detached_by_entity_deletion'`).Scan(&auditCount); err != nil || auditCount != len(devices) {
		t.Fatalf("detach audit count=%d err=%v", auditCount, err)
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check found a violation after deletion")
	}
	var untouchedStatus string
	if err := db.QueryRow(`SELECT status FROM devices WHERE id='already-detached'`).Scan(&untouchedStatus); err != nil || untouchedStatus != "detached" {
		t.Fatalf("unassociated detached device status=%q err=%v", untouchedStatus, err)
	}
}

func TestDeleteEntityDeviceDetachmentRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	store, _ := openDeviceDeleteTestStore(t)
	db := store.DB()
	entityID := "10000000-0000-4000-8000-000000000202"
	createDeleteTestEntity(t, store, entityID)
	seedDeleteDevice(t, db, entityID, "device-a", "active", false)
	seedDeleteDevice(t, db, entityID, "device-b", "active", false)
	if _, err := db.Exec(`CREATE TRIGGER fail_second_device_detach BEFORE UPDATE ON devices WHEN OLD.id='device-b' BEGIN SELECT RAISE(ABORT,'injected detach failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, entityID); err == nil {
		t.Fatal("expected injected device detach failure")
	}
	var entityCount, attachedCount, activeCredentialCount, detachAuditCount, deleteActivityCount int
	for _, check := range []struct {
		query string
		args  []any
		dst   *int
	}{
		{`SELECT count(*) FROM entities WHERE id=?`, []any{entityID}, &entityCount},
		{`SELECT count(*) FROM devices WHERE entity_id=? AND status='active'`, []any{entityID}, &attachedCount},
		{`SELECT count(*) FROM device_credentials c JOIN devices d ON d.id=c.device_id WHERE d.entity_id=? AND c.revoked_at IS NULL`, []any{entityID}, &activeCredentialCount},
		{`SELECT count(*) FROM admin_audit_events WHERE action='device.detached_by_entity_deletion'`, nil, &detachAuditCount},
		{`SELECT count(*) FROM activity_events WHERE entity_id=? AND event_type='entity.deleted'`, []any{entityID}, &deleteActivityCount},
	} {
		if err := db.QueryRow(check.query, check.args...).Scan(check.dst); err != nil {
			t.Fatal(err)
		}
	}
	if entityCount != 1 || attachedCount != 2 || activeCredentialCount != 2 || detachAuditCount != 0 || deleteActivityCount != 0 {
		t.Fatalf("partial deletion survived rollback: entity=%d attached=%d credentials=%d detach_audit=%d delete_activity=%d", entityCount, attachedCount, activeCredentialCount, detachAuditCount, deleteActivityCount)
	}
}
