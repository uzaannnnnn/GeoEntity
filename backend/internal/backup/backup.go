package backup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"geoentity/backend/internal/repository"

	"modernc.org/sqlite"
)

type Manifest struct {
	FormatVersion int    `json:"format_version"`
	Application   string `json:"application"`
	CreatedAt     string `json:"created_at"`
	SchemaVersion int    `json:"schema_version"`
	DatabaseFile  string `json:"database_file"`
	SizeBytes     int64  `json:"size_bytes"`
	SHA256        string `json:"sha256"`
}

func Create(ctx context.Context, db *sql.DB, directory string, keep int) (Manifest, error) {
	if db == nil || directory == "" {
		return Manifest{}, errors.New("database and backup directory are required")
	}
	if keep < 1 {
		keep = 10
	}
	schemaVersion, err := databaseSchemaVersion(ctx, db)
	if err != nil {
		return Manifest{}, fmt.Errorf("read database schema version: %w", err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return Manifest{}, err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return Manifest{}, err
	}
	if err := recordBackupRequest(ctx, db); err != nil {
		return Manifest{}, fmt.Errorf("record backup audit event: %w", err)
	}
	now := time.Now().UTC()
	stamp := now.Format("20060102T150405.000000000Z")
	name := "geoentity-" + stamp + ".sqlite"
	stage := filepath.Join(directory, "."+name+".partial")
	final := filepath.Join(directory, name)
	if err := os.Remove(stage); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, err
	}
	if err := onlineBackup(ctx, db, stage); err != nil {
		_ = os.Remove(stage)
		return Manifest{}, err
	}
	if err := verifyFile(ctx, stage); err != nil {
		_ = os.Remove(stage)
		return Manifest{}, fmt.Errorf("backup verification: %w", err)
	}
	size, checksum, err := fileDigest(stage)
	if err != nil {
		_ = os.Remove(stage)
		return Manifest{}, err
	}
	if err = os.Chmod(stage, 0o600); err != nil {
		_ = os.Remove(stage)
		return Manifest{}, err
	}
	if err = os.Rename(stage, final); err != nil {
		_ = os.Remove(stage)
		return Manifest{}, err
	}
	if d, e := os.Open(directory); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	m := Manifest{FormatVersion: 1, Application: "GeoEntity", CreatedAt: now.Format(time.RFC3339Nano), SchemaVersion: schemaVersion, DatabaseFile: name, SizeBytes: size, SHA256: checksum}
	if err = writeManifest(filepath.Join(directory, name+".manifest.json"), m); err != nil {
		_ = os.Remove(final)
		return Manifest{}, err
	}
	if err = prune(directory, keep); err != nil {
		return m, err
	}
	return m, nil
}

func recordBackupRequest(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_events(id,occurred_at,actor_id,actor_label,action,resource_type,payload_version,details) VALUES(?,?,?,?,?,?,1,'{}')`, hex.EncodeToString(id), time.Now().UTC().Format(time.RFC3339Nano), "1", "administrator", "backup.requested", "backup"); err != nil {
		return err
	}
	return tx.Commit()
}

func List(directory string) ([]Manifest, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []Manifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Manifest{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".manifest.json") {
			continue
		}
		m, err := readManifest(filepath.Join(directory, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, m)
	}
	return out, nil
}
func Verify(ctx context.Context, manifestPath string) (Manifest, error) {
	m, err := readManifest(manifestPath)
	if err != nil {
		return m, err
	}
	dbPath := filepath.Join(filepath.Dir(manifestPath), m.DatabaseFile)
	size, sum, err := fileDigest(dbPath)
	if err != nil {
		return m, err
	}
	if size != m.SizeBytes || sum != m.SHA256 {
		return m, errors.New("backup checksum or size mismatch")
	}
	actualSchemaVersion, err := verifyFileSchema(ctx, dbPath)
	if err != nil {
		return m, err
	}
	if m.SchemaVersion != actualSchemaVersion {
		return m, fmt.Errorf("backup manifest schema version %d does not match database schema version %d", m.SchemaVersion, actualSchemaVersion)
	}
	return m, nil
}

func Restore(ctx context.Context, dbPath, manifestPath string) error {
	return restoreWithRename(ctx, dbPath, manifestPath, os.Rename)
}

func restoreWithRename(ctx context.Context, dbPath, manifestPath string, rename func(string, string) error) error {
	if dbPath == "" {
		return errors.New("database path is required")
	}
	m, err := Verify(ctx, manifestPath)
	if err != nil {
		return fmt.Errorf("backup rejected: %w", err)
	}
	active, err := filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(active)
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	candidate := active + ".restore-" + stamp
	if err = restoreSnapshot(ctx, filepath.Join(filepath.Dir(manifestPath), m.DatabaseFile), candidate); err != nil {
		_ = os.Remove(candidate)
		return err
	}
	if err = os.Chmod(candidate, 0o600); err != nil {
		_ = os.Remove(candidate)
		return err
	}
	if err = verifyFile(ctx, candidate); err != nil {
		_ = os.Remove(candidate)
		return fmt.Errorf("restore candidate verification: %w", err)
	}
	if err = revokeSessions(ctx, candidate); err != nil {
		_ = os.Remove(candidate)
		return err
	}
	if err = verifyFile(ctx, candidate); err != nil {
		_ = os.Remove(candidate)
		return err
	}
	rollback := active + ".rollback-" + stamp + ".sqlite"
	if _, err := os.Stat(active); err == nil {
		current, openErr := sql.Open("sqlite", active)
		if openErr != nil {
			return openErr
		}
		current.SetMaxOpenConns(1)
		if err = onlineBackup(ctx, current, rollback); err == nil {
			err = os.Chmod(rollback, 0o600)
		}
		if err == nil {
			err = verifyFile(ctx, rollback)
		}
		closeErr := current.Close()
		if err != nil {
			_ = os.Remove(rollback)
			_ = os.Remove(candidate)
			return fmt.Errorf("rollback snapshot failed: %w", err)
		}
		if closeErr != nil {
			_ = os.Remove(rollback)
			_ = os.Remove(candidate)
			return closeErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(candidate)
		return err
	}
	old := active + ".pre-restore-" + stamp
	hadCurrent := false
	var movedSidecars []string
	restoreOriginal := func() error {
		if !hadCurrent {
			return nil
		}
		if err := rename(old, active); err != nil {
			return err
		}
		for _, suffix := range movedSidecars {
			if err := rename(old+suffix, active+suffix); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err = os.Stat(active); err == nil {
		if err = rename(active, old); err != nil {
			_ = os.Remove(candidate)
			return err
		}
		hadCurrent = true
		for _, suffix := range []string{"-wal", "-shm"} {
			sourceSidecar, oldSidecar := active+suffix, old+suffix
			if _, statErr := os.Stat(sourceSidecar); statErr == nil {
				if moveErr := rename(sourceSidecar, oldSidecar); moveErr != nil {
					if rollbackErr := restoreOriginal(); rollbackErr != nil {
						return fmt.Errorf("move restore sidecar failed: %v; rollback failed: %w", moveErr, rollbackErr)
					}
					_ = os.Remove(candidate)
					return moveErr
				}
				movedSidecars = append(movedSidecars, suffix)
			} else if !errors.Is(statErr, os.ErrNotExist) {
				if rollbackErr := restoreOriginal(); rollbackErr != nil {
					return fmt.Errorf("inspect database sidecar failed: %v; rollback failed: %w", statErr, rollbackErr)
				}
				for _, moved := range movedSidecars {
					_ = os.Remove(old + moved)
				}
				_ = os.Remove(candidate)
				return statErr
			}
		}
	}
	if err = rename(candidate, active); err != nil {
		if rollbackErr := restoreOriginal(); rollbackErr != nil {
			return fmt.Errorf("install restore failed: %v; rollback failed: %w", err, rollbackErr)
		}
		_ = os.Remove(candidate)
		return fmt.Errorf("install restore failed; previous database preserved: %w", err)
	}
	if err = verifyFile(ctx, active); err != nil {
		_ = os.Remove(active)
		if rollbackErr := restoreOriginal(); rollbackErr != nil {
			return fmt.Errorf("installed restore failed verification: %v; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("installed database failed verification; previous database restored: %w", err)
	}
	if hadCurrent {
		_ = os.Remove(old)
		for _, suffix := range movedSidecars {
			_ = os.Remove(old + suffix)
		}
	}
	return nil
}

func revokeSessions(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM web_sessions`); err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_events(id,occurred_at,actor_id,actor_label,action,resource_type,payload_version,details) VALUES(?,?,?,?,?,?,1,'{}')`, hex.EncodeToString(id), time.Now().UTC().Format(time.RFC3339Nano), "1", "administrator", "database.restored", "database"); err != nil {
		return err
	}
	return tx.Commit()
}

func onlineBackup(ctx context.Context, source *sql.DB, destination string) error {
	conn, err := source.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	uri := destination
	return conn.Raw(func(raw any) error {
		driver, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("sqlite driver does not expose Online Backup API")
		}
		backup, err := driver.NewBackup(uri)
		if err != nil {
			return err
		}
		more := true
		for more {
			more, err = backup.Step(128)
			if err != nil {
				_ = backup.Finish()
				return err
			}
			select {
			case <-ctx.Done():
				_ = backup.Finish()
				return ctx.Err()
			default:
			}
		}
		return backup.Finish()
	})
}
func restoreSnapshot(ctx context.Context, source, destination string) error {
	db, err := sql.Open("sqlite", destination)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		driver, ok := raw.(interface {
			NewRestore(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("sqlite driver does not expose Online Restore API")
		}
		backup, err := driver.NewRestore(source)
		if err != nil {
			return err
		}
		more := true
		for more {
			more, err = backup.Step(128)
			if err != nil {
				_ = backup.Finish()
				return err
			}
			select {
			case <-ctx.Done():
				_ = backup.Finish()
				return ctx.Err()
			default:
			}
		}
		return backup.Finish()
	})
}
func fileURI(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String()
}

func verifyFile(ctx context.Context, path string) error {
	_, err := verifyFileSchema(ctx, path)
	return err
}

func databaseSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, errors.New("database is required")
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	if _, err := repository.VerifyMigrationLedgerVersion(ctx, db, version); err != nil {
		return 0, err
	}
	return version, nil
}

func verifyFileSchema(ctx context.Context, path string) (int, error) {
	db, err := sql.Open("sqlite", fileURI(path)+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return 0, err
	}
	integrity := []string{}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			rows.Close()
			return 0, err
		}
		integrity = append(integrity, s)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(integrity) != 1 || integrity[0] != "ok" {
		return 0, fmt.Errorf("SQLite integrity_check failed: %v", integrity)
	}
	rows, err = db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return 0, err
	}
	hasFK := rows.Next()
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return 0, rowErr
	}
	if hasFK {
		return 0, errors.New("SQLite foreign_key_check found a violation")
	}
	schemaVersion, err := databaseSchemaVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	triggerNames := []string{"admin_audit_events_no_update", "admin_audit_events_no_delete", "geofence_events_no_update", "geofence_events_no_delete"}
	if schemaVersion >= 7 {
		triggerNames = append(triggerNames, "device_sensors_no_update", "device_sensors_no_delete", "sensor_observations_validate_insert", "sensor_observations_no_update", "sensor_observations_no_delete")
	}
	for _, name := range triggerNames {
		var n int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name=?`, name).Scan(&n); err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, fmt.Errorf("required immutable trigger %s is missing", name)
		}
	}
	return schemaVersion, nil
}
func fileDigest(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}
func writeManifest(path string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".partial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
func readManifest(path string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.FormatVersion != 1 || m.Application != "GeoEntity" || (m.SchemaVersion != 6 && m.SchemaVersion != 7) || filepath.Base(m.DatabaseFile) != m.DatabaseFile || !strings.HasSuffix(m.DatabaseFile, ".sqlite") || m.SizeBytes <= 0 || len(m.SHA256) != 64 {
		return m, errors.New("invalid backup manifest")
	}
	return m, nil
}
func prune(dir string, keep int) error {
	manifests, err := List(dir)
	if err != nil {
		return err
	}
	if len(manifests) <= keep {
		return nil
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].CreatedAt > manifests[j].CreatedAt })
	for _, m := range manifests[keep:] {
		_ = os.Remove(filepath.Join(dir, m.DatabaseFile))
		_ = os.Remove(filepath.Join(dir, m.DatabaseFile+".manifest.json"))
	}
	return nil
}
