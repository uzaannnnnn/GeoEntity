package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func TestMigration007FreshAndPopulatedV26Upgrade(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "populated-v26.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	fixture := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Name() > "006_admin_security.sql" {
			continue
		}
		body, e := fs.ReadFile(migrations, "migrations/"+entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		fixture["migrations/"+entry.Name()] = &fstest.MapFile{Data: body}
	}
	if err = MigrateFS(ctx, db, fixture); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO entities(id,name,type,status,latitude,longitude,created_at,updated_at) VALUES('10000000-0000-4000-8000-000000000019','V2.6 fixture','facility','active',1,2,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	before := []string{}
	rows, err := db.Query(`SELECT checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var sum string
		if err = rows.Scan(&sum); err != nil {
			t.Fatal(err)
		}
		before = append(before, sum)
	}
	rows.Close()
	if len(before) != 6 {
		t.Fatalf("V2.6 fixture ledger size=%d", len(before))
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	version, err := VerifyMigrationLedger(ctx, db)
	if err != nil || version != 7 {
		t.Fatalf("upgraded ledger=%d err=%v", version, err)
	}
	var name string
	if err = db.QueryRow(`SELECT name FROM entities WHERE id='10000000-0000-4000-8000-000000000019'`).Scan(&name); err != nil || name != "V2.6 fixture" {
		t.Fatalf("preserved entity=%q err=%v", name, err)
	}
	rows, err = db.Query(`SELECT checksum FROM schema_migrations WHERE version<=6 ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	after := []string{}
	for rows.Next() {
		var sum string
		if err = rows.Scan(&sum); err != nil {
			t.Fatal(err)
		}
		after = append(after, sum)
	}
	rows.Close()
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("001-006 checksums changed: %v => %v", before, after)
	}
	for _, name := range []string{"devices", "device_credentials", "device_sensors", "sensor_observations"} {
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s count=%d err=%v", name, n, err)
		}
	}
	for _, name := range []string{"device_memberships", "device_events", "geofence_memberships", "geofence_events_v2"} {
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil || n != 0 {
			t.Errorf("out-of-scope table %s count=%d err=%v", name, n, err)
		}
	}
}
