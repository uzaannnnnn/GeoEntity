package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func migrationPrefix(t *testing.T, last int) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	for version := 1; version <= last; version++ {
		var name string
		switch version {
		case 1:
			name = "001_create_entities.sql"
		case 2:
			name = "002_advanced_entity_management.sql"
		case 3:
			name = "003_entity_search_and_pagination.sql"
		case 4:
			name = "004_geofences.sql"
		case 5:
			name = "005_geofence_memberships_and_events.sql"
		}
		body, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		files["migrations/"+name] = &fstest.MapFile{Data: body}
	}
	return files
}

func TestMigration005UpgradesPopulatedV25BAndPreservesEarlierChecksums(t *testing.T) {
	ctx := context.Background()
	db := newMigrationDB(t)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	seedLegacy(t, db)
	if err := MigrateFS(ctx, db, migrationPrefix(t, 4)); err != nil {
		t.Fatal(err)
	}
	const geofenceID = "00000000-0000-4000-8000-000000000301"
	if _, err := db.Exec(`INSERT INTO geofences(id,name,normalized_name,enabled,geometry_type,geometry_json,created_at,updated_at) VALUES(?,?,?,1,'Circle',?,?,?)`, geofenceID, "Existing", "existing", `{"type":"Circle","center":[1,2],"radius_m":300}`, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	var beforeChecksums []string
	rows, err := db.Query(`SELECT checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var sum string
		if err := rows.Scan(&sum); err != nil {
			t.Fatal(err)
		}
		beforeChecksums = append(beforeChecksums, sum)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	beforeEntities := entitySnapshot(t, db)
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db); fmt.Sprint(got) != fmt.Sprint(beforeEntities) {
		t.Fatalf("migration changed entities: before=%v after=%v", beforeEntities, got)
	}
	var definitions int
	if err := db.QueryRow(`SELECT count(*) FROM geofences WHERE id=?`, geofenceID).Scan(&definitions); err != nil || definitions != 1 {
		t.Fatalf("existing geofence count=%d err=%v", definitions, err)
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2 3 4 5 6 7]" {
		t.Fatalf("migration ledger=%v", got)
	}
	rows, err = db.Query(`SELECT checksum FROM schema_migrations WHERE version<=4 ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	var afterChecksums []string
	for rows.Next() {
		var sum string
		if err := rows.Scan(&sum); err != nil {
			t.Fatal(err)
		}
		afterChecksums = append(afterChecksums, sum)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(afterChecksums) != fmt.Sprint(beforeChecksums) {
		t.Fatalf("earlier migration checksums changed: before=%v after=%v", beforeChecksums, afterChecksums)
	}
	for _, name := range []string{"entity_geofence_memberships", "geofence_events", "idx_entity_geofence_memberships_geofence_state_entity", "idx_geofence_events_geofence_time_id", "idx_geofence_events_entity_time_id", "geofence_events_no_update", "geofence_events_no_delete"} {
		var found int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, name).Scan(&found); err != nil || found != 1 {
			t.Errorf("migration object %q found=%d err=%v", name, found, err)
		}
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migration rerun: %v", err)
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2 3 4 5 6 7]" {
		t.Fatalf("rerun ledger=%v", got)
	}
}

func TestFreshDatabaseAppliesMigrations001Through007(t *testing.T) {
	db := newMigrationDB(t)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2 3 4 5 6 7]" {
		t.Fatalf("fresh migration ledger=%v", got)
	}
	for _, name := range []string{"geofences", "entity_geofence_memberships", "geofence_events"} {
		var found int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil || found != 1 {
			t.Errorf("fresh schema table %q found=%d err=%v", name, found, err)
		}
	}
}

func TestOpenBackfillsMembershipsForExistingEntitiesAndDefinitions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "existing-v25b.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	seedLegacy(t, db)
	if err := MigrateFS(ctx, db, migrationPrefix(t, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO geofences(id,name,normalized_name,enabled,geometry_type,geometry_json,created_at,updated_at) VALUES('00000000-0000-4000-8000-000000000401','Legacy zone','legacy zone',1,'Circle','{"type":"Circle","center":[2,1],"radius_m":1000}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	inside, err := store.EntityGeofences(ctx, "legacy-id-a")
	if err != nil || len(inside) != 1 || inside[0].State != "inside" {
		t.Fatalf("backfilled inside membership=%+v err=%v", inside, err)
	}
	outside, err := store.EntityGeofences(ctx, "legacy-id-b")
	if err != nil || len(outside) != 1 || outside[0].State != "outside" {
		t.Fatalf("backfilled outside membership=%+v err=%v", outside, err)
	}
	page, err := store.GeofenceEvents(ctx, "00000000-0000-4000-8000-000000000401", GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 0 {
		t.Fatalf("migration backfill fabricated events: %+v err=%v", page, err)
	}
}

func TestMigration005FailureRollsBackItsTablesAndLedger(t *testing.T) {
	db := newMigrationDB(t)
	seedLegacy(t, db)
	if err := MigrateFS(context.Background(), db, migrationPrefix(t, 4)); err != nil {
		t.Fatal(err)
	}
	files := migrationPrefix(t, 5)
	body := append(append([]byte(nil), files["migrations/005_geofence_memberships_and_events.sql"].Data...), []byte("\nCREATE TABLE injected_failure_marker(id INTEGER); INSERT INTO missing_table(id) VALUES(1);\n")...)
	files["migrations/005_geofence_memberships_and_events.sql"] = &fstest.MapFile{Data: body}
	if err := MigrateFS(context.Background(), db, files); err == nil {
		t.Fatal("expected migration failure")
	}
	for _, name := range []string{"entity_geofence_memberships", "geofence_events", "injected_failure_marker"} {
		var found int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil || found != 0 {
			t.Errorf("failed migration left table %q (found=%d err=%v)", name, found, err)
		}
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2 3 4]" {
		t.Fatalf("failed migration changed ledger: %v", got)
	}
}
