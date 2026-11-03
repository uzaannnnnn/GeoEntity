package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"geoentity/backend/internal/entity"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("entity not found")

//go:embed migrations/*.sql
var migrations embed.FS

type SQLite struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection keeps in-memory databases deterministic and SQLite
	// writes serialized for this small MVP service.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	store := &SQLite{db: db}
	if err := store.backfillMissingGeofenceMemberships(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize geofence memberships: %w", err)
	}
	return store, nil
}

func (r *SQLite) Close() error { return r.db.Close() }

func Migrate(ctx context.Context, db *sql.DB) error {
	return MigrateFS(ctx, db, migrations)
}

type migration struct {
	version  int
	name     string
	sql      []byte
	checksum string
}

var migrationFile = regexp.MustCompile(`^([0-9]{3,})_(.+)\.sql$`)

func loadMigrations(source fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(source, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}
	var result []migration
	seen := map[int]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationFile.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, _ := strconv.Atoi(match[1])
		if version < 1 || seen[version] {
			return nil, fmt.Errorf("duplicate or invalid migration version %d", version)
		}
		seen[version] = true
		body, err := fs.ReadFile(source, filepath.ToSlash(filepath.Join("migrations", entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(body)
		result = append(result, migration{version: version, name: entry.Name(), sql: body, checksum: hex.EncodeToString(digest[:])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for i, item := range result {
		if item.version != i+1 {
			return nil, fmt.Errorf("migration sequence has a gap before version %d", item.version)
		}
	}
	if len(result) == 0 || result[0].version != 1 || result[0].name != "001_create_entities.sql" {
		return nil, errors.New("migration 001_create_entities.sql is required as the V1 baseline")
	}
	return result, nil
}

// MigrateFS is exposed for deterministic migration tests; production uses embedded migrations.
func MigrateFS(ctx context.Context, db *sql.DB, source fs.FS) error {
	items, err := loadMigrations(source)
	if err != nil {
		return err
	}
	legacySQL, err := fs.ReadFile(source, "migrations/001_create_entities.sql")
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration inspection: %w", err)
	}
	defer tx.Rollback()
	ledgerExists, err := tableExists(ctx, tx, "schema_migrations")
	if err != nil {
		return err
	}
	if !ledgerExists {
		entitiesExists, err := tableExists(ctx, tx, "entities")
		if err != nil {
			return err
		}
		if entitiesExists {
			if err := verifyLegacyV1(ctx, tx, legacySQL); err != nil {
				return fmt.Errorf("legacy database rejected: %w", err)
			}
		} else {
			objects, err := userObjects(ctx, tx)
			if err != nil {
				return err
			}
			if len(objects) != 0 {
				return fmt.Errorf("database has no migration ledger but contains unexpected schema objects: %s", strings.Join(objects, ", "))
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
			return fmt.Errorf("create migration ledger: %w", err)
		}
		if !entitiesExists {
			if _, err := tx.ExecContext(ctx, string(items[0].sql)); err != nil {
				return fmt.Errorf("apply migration %s: %w", items[0].name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)`, items[0].version, items[0].name, items[0].checksum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record V1 baseline: %w", err)
		}
	} else if err := verifyLedgerSchema(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration baseline: %w", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT version,name,checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	applied := map[int]bool{}
	expectedVersion := 1
	for rows.Next() {
		var version int
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			return err
		}
		if version < 1 || version > len(items) {
			rows.Close()
			return fmt.Errorf("migration ledger contains unknown version %d", version)
		}
		if version != expectedVersion {
			rows.Close()
			return fmt.Errorf("migration ledger is missing version %d before version %d", expectedVersion, version)
		}
		item := items[version-1]
		if item.version != version || item.name != name {
			rows.Close()
			return fmt.Errorf("migration ledger name mismatch for version %d", version)
		}
		if item.checksum != checksum {
			rows.Close()
			return fmt.Errorf("migration checksum mismatch for version %d (%s)", version, name)
		}
		applied[version] = true
		expectedVersion++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		if applied[item.version] {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %03d: %w", item.version, err)
		}
		if _, err = tx.ExecContext(ctx, string(item.sql)); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)`, item.version, item.name, item.checksum, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %03d (%s): %w", item.version, item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %03d (%s): %w", item.version, item.name, err)
		}
	}
	return nil
}

type dbQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func tableExists(ctx context.Context, q dbQueryer, name string) (bool, error) {
	var found int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found)
	return found != 0, err
}

func userObjects(ctx context.Context, q dbQueryer) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT type || ':' || name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func normalizeSQL(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func verifyLegacyV1(ctx context.Context, q dbQueryer, source []byte) error {
	objects, err := userObjects(ctx, q)
	if err != nil {
		return err
	}
	expected := []string{"index:idx_entities_created_at", "table:entities"}
	if strings.Join(objects, "|") != strings.Join(expected, "|") {
		return fmt.Errorf("expected only V1 entities table and idx_entities_created_at; found %s", strings.Join(objects, ", "))
	}
	var actual string
	if err := q.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='entities'`).Scan(&actual); err != nil {
		return fmt.Errorf("read entities schema: %w", err)
	}
	want := strings.SplitN(string(source), ";", 2)[0]
	want = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS`).ReplaceAllString(want, "CREATE TABLE")
	if normalizeSQL(actual) != normalizeSQL(want) {
		return errors.New("entities table definition does not match known V1 schema")
	}
	var indexSQL string
	if err := q.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_entities_created_at'`).Scan(&indexSQL); err != nil {
		return errors.New("required V1 index idx_entities_created_at is missing")
	}
	wantIndex := "CREATE INDEX idx_entities_created_at ON entities(created_at, id)"
	if normalizeSQL(indexSQL) != normalizeSQL(wantIndex) {
		return errors.New("idx_entities_created_at definition does not match known V1 schema")
	}
	var columns []string
	rows, err := q.QueryContext(ctx, `PRAGMA table_info(entities)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var dflt any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &dflt, &pk); err != nil {
			return err
		}
		columns = append(columns, name+":"+strings.ToUpper(kind)+":"+strconv.Itoa(notnull)+":"+strconv.Itoa(pk))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	wantColumns := []string{"id:TEXT:1:1", "name:TEXT:1:0", "type:TEXT:1:0", "description:TEXT:1:0", "status:TEXT:1:0", "latitude:REAL:1:0", "longitude:REAL:1:0", "created_at:TEXT:1:0", "updated_at:TEXT:1:0"}
	if strings.Join(columns, "|") != strings.Join(wantColumns, "|") {
		return fmt.Errorf("entities columns do not match V1 schema: found %s", strings.Join(columns, ", "))
	}
	return nil
}

func verifyLedgerSchema(ctx context.Context, q dbQueryer) error {
	var sqlText string
	if err := q.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&sqlText); err != nil {
		return fmt.Errorf("read schema_migrations definition: %w", err)
	}
	want := `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`
	if normalizeSQL(sqlText) != normalizeSQL(want) {
		return errors.New("schema_migrations table has an unexpected definition")
	}
	return nil
}

func (r *SQLite) List(ctx context.Context) ([]entity.Entity, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, description, status, latitude, longitude, created_at, updated_at FROM entities ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]entity.Entity, 0)
	for rows.Next() {
		item, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLite) Get(ctx context.Context, id string) (entity.Entity, error) {
	item, err := scanEntity(r.db.QueryRowContext(ctx, `SELECT id, name, type, description, status, latitude, longitude, created_at, updated_at FROM entities WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Entity{}, ErrNotFound
	}
	return item, err
}

func (r *SQLite) Create(ctx context.Context, item entity.Entity) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO entities (id, name, type, description, status, latitude, longitude, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.Name, item.Type, item.Description, item.Status, item.Latitude, item.Longitude, item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if err = insertActivity(ctx, tx, item.ID, "entity.created", map[string]any{"entity": activityEntity(item)}); err != nil {
		return err
	}
	definitions, err := loadEnabledGeofencesTx(ctx, tx)
	if err != nil {
		return err
	}
	if err := establishEntityMembershipsTx(ctx, tx, item, definitions, item.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLite) Update(ctx context.Context, item entity.Entity) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	previous, err := scanEntity(tx.QueryRowContext(ctx, `SELECT id,name,type,description,status,latitude,longitude,created_at,updated_at FROM entities WHERE id=?`, item.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	evaluations, err := prepareEntityMembershipUpdatesTx(ctx, tx, previous, item)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE entities SET name=?,type=?,description=?,status=?,latitude=?,longitude=?,updated_at=? WHERE id=?`, item.Name, item.Type, item.Description, item.Status, item.Latitude, item.Longitude, item.UpdatedAt.UTC().Format(time.RFC3339Nano), item.ID); err != nil {
		return err
	}
	if err := persistEntityMembershipUpdatesTx(ctx, tx, item, evaluations); err != nil {
		return err
	}
	if err = insertActivity(ctx, tx, item.ID, "entity.updated", map[string]any{"before": activityEntity(previous), "after": activityEntity(item)}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLite) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	item, err := scanEntity(tx.QueryRowContext(ctx, `SELECT id,name,type,description,status,latitude,longitude,created_at,updated_at FROM entities WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	snapshot := map[string]any{"entity": activityEntity(item)}
	for _, spec := range []struct {
		key, query string
		keyed      bool
	}{
		{"attributes", `SELECT d.key,CASE d.value_type WHEN 'string' THEN v.text_value WHEN 'number' THEN v.number_value ELSE v.boolean_value END FROM entity_attribute_values v JOIN attribute_definitions d ON d.id=v.definition_id WHERE v.entity_id=? ORDER BY d.key`, true},
		{"tags", `SELECT t.name FROM entity_tags x JOIN tags t ON t.id=x.tag_id WHERE x.entity_id=? ORDER BY t.normalized_name`, false},
		{"categories", `SELECT c.name FROM entity_categories x JOIN categories c ON c.id=x.category_id WHERE x.entity_id=? ORDER BY c.normalized_name`, false},
	} {
		var values any
		if spec.keyed {
			values = map[string]any{}
		} else {
			values = []any{}
		}
		rows, e := tx.QueryContext(ctx, spec.query, id)
		if e != nil {
			return e
		}
		for rows.Next() {
			var k string
			var v any
			if spec.keyed {
				if e = rows.Scan(&k, &v); e != nil {
					rows.Close()
					return e
				}
				values.(map[string]any)[k] = v
			} else {
				if e = rows.Scan(&v); e != nil {
					rows.Close()
					return e
				}
				values = append(values.([]any), v)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		snapshot[spec.key] = values
	}
	if err = insertActivity(ctx, tx, id, "entity.deleted", snapshot); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM entities WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

type scanner interface{ Scan(...any) error }

func scanEntity(row scanner) (entity.Entity, error) {
	var item entity.Entity
	var createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Status, &item.Latitude, &item.Longitude, &createdAt, &updatedAt); err != nil {
		return entity.Entity{}, err
	}
	var err error
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return entity.Entity{}, err
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return entity.Entity{}, err
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}
