package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

const v1Migration = `CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL CHECK(length(trim(name)) > 0),
    type TEXT NOT NULL CHECK(type IN ('vehicle', 'iot_device', 'facility', 'other')),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('active', 'inactive', 'maintenance')),
    latitude REAL NOT NULL CHECK(latitude >= -90 AND latitude <= 90),
    longitude REAL NOT NULL CHECK(longitude >= -180 AND longitude <= 180),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_entities_created_at ON entities(created_at, id);`

func migrationFS(v2 string) fstest.MapFS {
	files := fstest.MapFS{"migrations/001_create_entities.sql": &fstest.MapFile{Data: []byte(v1Migration)}}
	if v2 != "" {
		files["migrations/002_foundation.sql"] = &fstest.MapFile{Data: []byte(v2)}
	}
	return files
}

func newMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedLegacy(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(v1Migration); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"legacy-id-a", "legacy-id-b"} {
		_, err := db.Exec(`INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, fmt.Sprintf("Fixture %d", i), "facility", fmt.Sprintf("description %d", i), "active", float64(i+1), float64(i+2), "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func entitySnapshot(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT id,name,type,description,status,latitude,longitude,created_at,updated_at FROM entities ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var id, name, kind, description, status, created, updated string
		var lat, lng float64
		if err := rows.Scan(&id, &name, &kind, &description, &status, &lat, &lng, &created, &updated); err != nil {
			t.Fatal(err)
		}
		values = append(values, fmt.Sprintf("%s|%s|%s|%s|%s|%g|%g|%s|%s", id, name, kind, description, status, lat, lng, created, updated))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return values
}

func ledgerVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestMigrateEmptyDatabaseAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "empty.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := MigrateFS(ctx, db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY)")); err != nil {
		t.Fatal(err)
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("ledger versions=%v", got)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigrateFS(ctx, db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY)")); err != nil {
		t.Fatalf("reopen migration: %v", err)
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("reopen ledger versions=%v", got)
	}
}

func TestMigrateRecognizesLegacyV1AndPreservesTwoRows(t *testing.T) {
	db := newMigrationDB(t)
	seedLegacy(t, db)
	before := entitySnapshot(t, db)
	if len(before) != 2 {
		t.Fatalf("fixture has %d records", len(before))
	}
	if err := MigrateFS(context.Background(), db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY)")); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatal("legacy rows changed during baseline registration")
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("ledger versions=%v", got)
	}
	if err := MigrateFS(context.Background(), db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY)")); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if got := entitySnapshot(t, db); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatal("rerun changed legacy rows")
	}
}

func TestMigrateRejectsChecksumMismatch(t *testing.T) {
	db := newMigrationDB(t)
	if err := MigrateFS(context.Background(), db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY)")); err != nil {
		t.Fatal(err)
	}
	err := MigrateFS(context.Background(), db, migrationFS("CREATE TABLE foundation_marker (id INTEGER PRIMARY KEY, value TEXT)"))
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestMigrateRejectsUnknownLegacySchema(t *testing.T) {
	db := newMigrationDB(t)
	if _, err := db.Exec(`CREATE TABLE entities(id TEXT PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	err := MigrateFS(context.Background(), db, migrationFS(""))
	if err == nil || !strings.Contains(err.Error(), "legacy database rejected") {
		t.Fatalf("expected fail-closed diagnostic, got %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='schema_migrations'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rejected database was modified")
	}
}

func TestMigrateRejectsLegacySchemaMissingIndexOrConstraints(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{name: "missing index", schema: strings.SplitN(v1Migration, ";", 2)[0]},
		{name: "missing checks", schema: strings.Replace(v1Migration, "CHECK(length(trim(name)) > 0)", "CHECK(name IS NOT NULL)", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newMigrationDB(t)
			if _, err := db.Exec(tt.schema); err != nil {
				t.Fatal(err)
			}
			err := MigrateFS(context.Background(), db, migrationFS(""))
			if err == nil || !strings.Contains(err.Error(), "legacy database rejected") {
				t.Fatalf("expected fail-closed legacy diagnostic, got %v", err)
			}
		})
	}
}

func TestMigrateRejectsUnrelatedSchemaWithoutLedger(t *testing.T) {
	db := newMigrationDB(t)
	if _, err := db.Exec(`CREATE TABLE unrelated(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	err := MigrateFS(context.Background(), db, migrationFS(""))
	if err == nil || !strings.Contains(err.Error(), "unexpected schema objects") {
		t.Fatalf("expected unexpected schema diagnostic, got %v", err)
	}
}

func TestFailedMigrationRollsBackSchemaAndLedgerEntry(t *testing.T) {
	db := newMigrationDB(t)
	seedLegacy(t, db)
	badMigration := "CREATE TABLE should_rollback (id INTEGER PRIMARY KEY); INSERT INTO missing_table(id) VALUES(1);"
	err := MigrateFS(context.Background(), db, migrationFS(badMigration))
	if err == nil {
		t.Fatal("expected migration failure")
	}
	if got := ledgerVersions(t, db); fmt.Sprint(got) != "[1]" {
		t.Fatalf("ledger after failure=%v", got)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='should_rollback'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration left a partial table")
	}
}

func TestExistingLedgerIsValidated(t *testing.T) {
	db := newMigrationDB(t)
	if err := MigrateFS(context.Background(), db, migrationFS("")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET name='wrong_name' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	err := MigrateFS(context.Background(), db, migrationFS(""))
	if err == nil || !strings.Contains(err.Error(), "name mismatch") {
		t.Fatalf("expected ledger consistency failure, got %v", err)
	}
}
