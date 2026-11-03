package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/entity"
)

func TestMigration006IsAdditiveAndAuditIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, name := range []string{"admin_users", "web_sessions", "admin_audit_events", "idx_web_sessions_expiry", "idx_admin_audit_events_time_id", "admin_audit_events_no_update", "admin_audit_events_no_delete"} {
		var n int
		if err = store.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("migration object %s count=%d err=%v", name, n, err)
		}
	}
	for _, name := range []string{"entity_geofence_memberships", "geofence_events", "geofence_events_no_update", "geofence_events_no_delete"} {
		var n int
		if err = store.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("V2.5 object %s count=%d err=%v", name, n, err)
		}
	}
	body, err := os.ReadFile(filepath.Join("migrations", "006_admin_security.sql"))
	if err != nil {
		t.Fatal(err)
	}
	migration := strings.ToLower(string(body))
	if strings.Contains(migration, "create table entity_geofence_memberships") || strings.Contains(migration, "create table geofence_events") {
		t.Fatal("migration 006 must not create V2.5 membership or event tables")
	}
	if version, err := VerifyMigrationLedger(ctx, store.DB()); err != nil || version != 6 {
		t.Fatalf("migration ledger version=%d err=%v", version, err)
	}
}

func TestBusinessMutationAndAuditAreAtomicAndActorAttributed(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC()
	item := entity.Entity{ID: "10000000-0000-4000-8000-000000000091", Name: "Audited", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: at, UpdatedAt: at}
	actorCtx := auth.WithPrincipal(ctx, auth.Principal{ID: "1", Label: "administrator"})
	if err = store.Create(actorCtx, item); err != nil {
		t.Fatal(err)
	}
	var action, actor string
	if err = store.DB().QueryRow(`SELECT action,actor_id FROM admin_audit_events WHERE resource_id=?`, item.ID).Scan(&action, &actor); err != nil || action != "entity.created" || actor != "1" {
		t.Fatalf("audit action=%s actor=%s err=%v", action, actor, err)
	}
	if _, err = store.DB().Exec(`UPDATE admin_audit_events SET action='rewritten' WHERE resource_id=?`, item.ID); err == nil {
		t.Fatal("audit update was allowed")
	}
	if _, err = store.DB().Exec(`DELETE FROM admin_audit_events WHERE resource_id=?`, item.ID); err == nil {
		t.Fatal("audit deletion was allowed")
	}
	if _, err = store.DB().Exec(`CREATE TRIGGER reject_admin_audit BEFORE INSERT ON admin_audit_events BEGIN SELECT RAISE(ABORT,'audit unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	item.ID = "10000000-0000-4000-8000-000000000092"
	item.Name = "Must rollback"
	item.CreatedAt = time.Now().UTC()
	item.UpdatedAt = item.CreatedAt
	if err = store.Create(actorCtx, item); err == nil {
		t.Fatal("business mutation succeeded while audit insert failed")
	}
	if _, err = store.Get(ctx, item.ID); err == nil {
		t.Fatal("entity remained after audit failure")
	}
}

func TestSystemMutationsDoNotForgeAdministratorAuditActor(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "direct.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM admin_audit_events`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("migration invented admin audit history: count=%d err=%v", n, err)
	}
}
