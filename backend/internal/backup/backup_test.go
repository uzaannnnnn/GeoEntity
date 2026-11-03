package backup

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
	"geoentity/backend/internal/repository"
	"modernc.org/sqlite"
)

func TestOnlineBackupWALRestoreRevokesSessionsAndPreservesRollback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "geoentity.db")
	store, err := repository.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var mode string
	if err = store.DB().QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal mode=%q", mode)
	}
	admin, err := auth.New(store.DB(), auth.Config{PublicOrigin: "https://geo.test", CookieSecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.Provision(ctx, "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = admin.Login(ctx, "127.0.0.1", "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	before := entity.Entity{ID: "10000000-0000-4000-8000-000000000001", Name: "Snapshot row", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.Create(ctx, before); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(root, "backups")
	manifest, err := Create(ctx, store.DB(), backupDir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 7 {
		t.Fatalf("schema-7 backup manifest version=%d", manifest.SchemaVersion)
	}
	manifestPath := filepath.Join(backupDir, manifest.DatabaseFile+".manifest.json")
	if st, err := os.Stat(backupDir); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("backup directory mode=%v err=%v", st, err)
	}
	if st, err := os.Stat(filepath.Join(backupDir, manifest.DatabaseFile)); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("backup file mode=%v err=%v", st, err)
	}
	after := entity.Entity{ID: "10000000-0000-4000-8000-000000000002", Name: "Post snapshot row", Type: "facility", Status: "active", Latitude: 3, Longitude: 4, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.Create(ctx, after); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = Restore(ctx, dbPath, manifestPath); err != nil {
		t.Fatal(err)
	}
	restored, err := repository.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	items, err := restored.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != before.ID {
		t.Fatalf("restored rows=%+v", items)
	}
	var sessions, restoreAudits int
	if err = restored.DB().QueryRow(`SELECT count(*) FROM web_sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err = restored.DB().QueryRow(`SELECT count(*) FROM admin_audit_events WHERE action='database.restored'`).Scan(&restoreAudits); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || restoreAudits != 1 {
		t.Fatalf("sessions=%d restore audit rows=%d", sessions, restoreAudits)
	}
	rollbacks, err := filepath.Glob(dbPath + ".rollback-*.sqlite")
	if err != nil || len(rollbacks) != 1 {
		t.Fatalf("rollback snapshots=%v err=%v", rollbacks, err)
	}
	if _, err = Verify(ctx, manifestPath); err != nil {
		t.Fatalf("published backup no longer verifies: %v", err)
	}
}

func TestBackupRejectsManifestSchemaMismatchInBothDirections(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := repository.Open(ctx, filepath.Join(root, "source-v7.db"))
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(root, "v7-backups")
	m, err := Create(ctx, store.DB(), backupDir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(backupDir, m.DatabaseFile+".manifest.json")
	wrong := m
	wrong.SchemaVersion = 6
	wrongManifest := filepath.Join(backupDir, "schema7-manifest-says6.manifest.json")
	if err = writeManifest(wrongManifest, wrong); err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(ctx, wrongManifest); err == nil || !strings.Contains(err.Error(), "manifest schema version 6 does not match database schema version 7") {
		t.Fatalf("schema-7/schema-6 mismatch error=%v", err)
	}
	// Restore must reject a mismatched manifest before changing an existing target.
	targetPath := filepath.Join(root, "target.db")
	target, err := repository.Open(ctx, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	marker := entity.Entity{ID: "10000000-0000-4000-8000-000000000098", Name: "Preserve target", Type: "facility", Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = target.Create(ctx, marker); err != nil {
		t.Fatal(err)
	}
	if err = target.Close(); err != nil {
		t.Fatal(err)
	}
	if err = Restore(ctx, targetPath, wrongManifest); err == nil {
		t.Fatal("restore accepted schema-mismatched manifest")
	}
	target, err = repository.Open(ctx, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = target.Get(ctx, marker.ID); err != nil {
		t.Fatalf("mismatched restore changed original target: %v", err)
	}
	_ = target.Close()

	// Make an isolated schema-6 database by removing only migration-7 objects.
	sixPath := filepath.Join(root, "source-v6.db")
	six, err := repository.Open(ctx, sixPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`DROP TRIGGER sensor_observations_no_update`, `DROP TRIGGER sensor_observations_no_delete`, `DROP TRIGGER device_sensors_no_update`, `DROP TRIGGER device_sensors_no_delete`, `DROP TRIGGER sensor_observations_validate_insert`, `DROP TABLE sensor_observations`, `DROP TABLE device_sensors`, `DROP TABLE device_credentials`, `DROP TABLE devices`, `DELETE FROM schema_migrations WHERE version=7`} {
		if _, err = six.DB().Exec(stmt); err != nil {
			t.Fatalf("make isolated schema-6 fixture (%s): %v", stmt, err)
		}
	}
	sixDir := filepath.Join(root, "v6-backups")
	if err = os.MkdirAll(sixDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sixFile := filepath.Join(sixDir, "schema6.sqlite")
	if err = onlineBackup(ctx, six.DB(), sixFile); err != nil {
		t.Fatal(err)
	}
	_ = six.Close()
	size, sum, err := fileDigest(sixFile)
	if err != nil {
		t.Fatal(err)
	}
	sixManifest := Manifest{FormatVersion: 1, Application: "GeoEntity", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), SchemaVersion: 7, DatabaseFile: filepath.Base(sixFile), SizeBytes: size, SHA256: sum}
	sixManifestPath := filepath.Join(sixDir, "schema6-manifest-says7.manifest.json")
	if err = writeManifest(sixManifestPath, sixManifest); err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(ctx, sixManifestPath); err == nil || !strings.Contains(err.Error(), "manifest schema version 7 does not match database schema version 6") {
		t.Fatalf("schema-6/schema-7 mismatch error=%v", err)
	}
	if _, err = Verify(ctx, manifestPath); err != nil {
		t.Fatalf("valid schema-7 backup failed after mismatch checks: %v", err)
	}
}

func TestBackupRejectsInvalidManifestSchemaMetadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := repository.Open(ctx, filepath.Join(root, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Create(ctx, store.DB(), filepath.Join(root, "backups"), 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	m.SchemaVersion = 5
	path := filepath.Join(root, "invalid-schema-manifest.json")
	if err = writeManifest(path, m); err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(ctx, path); err == nil || !strings.Contains(err.Error(), "invalid backup manifest") {
		t.Fatalf("invalid manifest schema metadata error=%v", err)
	}
}

func TestInvalidBackupRejectedBeforeChangingTarget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, err := repository.Open(ctx, filepath.Join(root, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	manifest, err := Create(ctx, source.DB(), filepath.Join(root, "backups"), 10)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "backups", manifest.DatabaseFile+".manifest.json")
	if err = os.WriteFile(filepath.Join(root, "backups", manifest.DatabaseFile), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.db")
	targetDB, err := repository.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if err = targetDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err = Restore(ctx, target, manifestPath); err == nil {
		t.Fatal("corrupt backup was accepted")
	}
	targetDB, err = repository.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	var n int
	if err = targetDB.DB().QueryRow(`SELECT count(*) FROM entities`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("target changed after rejected restore: count=%d err=%v", n, err)
	}
}

func TestRestoreSwapFailureRollsBackOriginalDatabase(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, err := repository.Open(ctx, filepath.Join(root, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Create(ctx, source.DB(), filepath.Join(root, "backups"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "backups", manifest.DatabaseFile+".manifest.json")
	target := filepath.Join(root, "target.db")
	existing, err := repository.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	item := entity.Entity{ID: "10000000-0000-4000-8000-000000000099", Name: "Keep original", Type: "facility", Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = existing.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err = existing.Close(); err != nil {
		t.Fatal(err)
	}
	failInstall := func(from, to string) error {
		if strings.Contains(from, ".restore-") {
			return os.ErrPermission
		}
		return os.Rename(from, to)
	}
	if err = restoreWithRename(ctx, target, manifestPath, failInstall); err == nil {
		t.Fatal("simulated candidate swap failure was ignored")
	}
	check, err := repository.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	if _, err = check.Get(ctx, item.ID); err != nil {
		t.Fatalf("original database was not restored after swap failure: %v", err)
	}
	rollbacks, err := filepath.Glob(target + ".rollback-*.sqlite")
	if err != nil || len(rollbacks) != 1 {
		t.Fatalf("rollback snapshots=%v err=%v", rollbacks, err)
	}
}

func TestOnlineBackupUsesSQLiteDriverAPI(t *testing.T) {
	ctx := context.Background()
	source, err := repository.Open(ctx, filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dst := filepath.Join(t.TempDir(), "dest.sqlite")
	if err = onlineBackup(ctx, source.DB(), dst); err != nil {
		t.Fatal(err)
	}
	if err = verifyFile(ctx, dst); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dst)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 7 {
		t.Fatalf("copied schema version=%d err=%v", version, err)
	}
}

func TestDriverBackupMinimalTemporarySQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(0)
	if _, err = db.Exec(`CREATE TABLE sample(value TEXT); INSERT INTO sample VALUES('safe')`); err != nil {
		t.Fatal(err)
	}
	var pages int
	if err = db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	t.Logf("temporary source page count %d", pages)
	dst := filepath.Join(t.TempDir(), "copy.sqlite")
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	err = conn.Raw(func(raw any) error {
		driver, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			t.Fatal("raw driver does not expose backup")
		}
		b, e := driver.NewBackup(dst)
		if e != nil {
			return e
		}
		more := true
		for more {
			more, e = b.Step(-1)
			if e != nil {
				_ = b.Finish()
				return e
			}
		}
		return b.Finish()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestV26Schema6BackupManifestVerifiesAndRestores(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "v26.sqlite")
	store, err := repository.Open(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`DROP TRIGGER sensor_observations_no_update`, `DROP TRIGGER sensor_observations_no_delete`, `DROP TRIGGER device_sensors_no_update`, `DROP TRIGGER device_sensors_no_delete`, `DROP TABLE sensor_observations`, `DROP TABLE device_sensors`, `DROP TABLE device_credentials`, `DROP TABLE devices`, `DELETE FROM schema_migrations WHERE version=7`} {
		if _, err = store.DB().Exec(stmt); err != nil {
			t.Fatalf("make isolated V2.6 fixture (%s): %v", stmt, err)
		}
	}
	backupDir := filepath.Join(root, "backups")
	if err = os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "v26.sqlite"
	dbFile := filepath.Join(backupDir, name)
	if err = onlineBackup(ctx, store.DB(), dbFile); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	size, sum, err := fileDigest(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{FormatVersion: 1, Application: "GeoEntity", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), SchemaVersion: 6, DatabaseFile: name, SizeBytes: size, SHA256: sum}
	manifestPath := dbFile + ".manifest.json"
	if err = writeManifest(manifestPath, m); err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(ctx, manifestPath); err != nil {
		t.Fatalf("V2.6 manifest verify: %v", err)
	}
	target := filepath.Join(root, "restored.sqlite")
	if err = Restore(ctx, target, manifestPath); err != nil {
		t.Fatalf("V2.6 backup restore: %v", err)
	}
	restored, err := repository.Open(ctx, target)
	if err != nil {
		t.Fatalf("V2.6 restore did not migrate forward: %v", err)
	}
	defer restored.Close()
	if version, err := repository.VerifyMigrationLedger(ctx, restored.DB()); err != nil || version != 7 {
		t.Fatalf("restored schema version=%d err=%v", version, err)
	}
}

func TestOnlineBackupSnapshotConsistentDuringConcurrentWALWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent.db")
	initial, err := repository.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = initial.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	var mode string
	if err = db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("WAL mode=%q err=%v", mode, err)
	}
	if _, err = db.Exec(`CREATE TABLE concurrent_state(id INTEGER PRIMARY KEY CHECK(id=1),count INTEGER NOT NULL);INSERT INTO concurrent_state VALUES(1,0);CREATE TABLE concurrent_events(n INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		for i := 1; i <= 500; i++ {
			tx, e := db.BeginTx(ctx, nil)
			if e != nil {
				writerDone <- e
				return
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO concurrent_events(n) VALUES(?)`, i); e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE concurrent_state SET count=? WHERE id=1`, i)
			}
			if e == nil {
				e = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if e != nil {
				writerDone <- e
				return
			}
			if i == 1 {
				close(started)
			}
			time.Sleep(time.Millisecond)
		}
		writerDone <- nil
	}()
	<-started
	dir := filepath.Join(t.TempDir(), "backups")
	m, err := Create(ctx, db, dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-writerDone; err != nil {
		t.Fatal(err)
	}
	snapshot, err := sql.Open("sqlite", filepath.Join(dir, m.DatabaseFile))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	var state, count int
	if err = snapshot.QueryRow(`SELECT count FROM concurrent_state WHERE id=1`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err = snapshot.QueryRow(`SELECT count(*) FROM concurrent_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if state != count {
		t.Fatalf("backup captured inconsistent transaction: state=%d events=%d", state, count)
	}
	if _, err = Verify(ctx, filepath.Join(dir, m.DatabaseFile+".manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRetentionPrunesOnlyAfterVerifiedSnapshots(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(ctx, filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := filepath.Join(t.TempDir(), "backups")
	for i := 0; i < 3; i++ {
		if _, err = Create(ctx, store.DB(), dir, 2); err != nil {
			t.Fatal(err)
		}
	}
	manifests, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 2 {
		t.Fatalf("retained %d manifests, want 2", len(manifests))
	}
	for _, m := range manifests {
		if _, err = Verify(ctx, filepath.Join(dir, m.DatabaseFile+".manifest.json")); err != nil {
			t.Fatalf("retained snapshot does not verify: %v", err)
		}
	}
}
