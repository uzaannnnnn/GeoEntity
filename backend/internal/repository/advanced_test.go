package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"geoentity/backend/internal/entity"
)

func TestV22RowsSurviveAdvancedMigrationAndRerun(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "populated.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	baseline, err := fs.ReadFile(migrations, "migrations/001_create_entities.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateFS(ctx, db, fstest.MapFS{"migrations/001_create_entities.sql": &fstest.MapFile{Data: baseline}}); err != nil {
		t.Fatalf("prepare V2.2 schema fixture: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO entities VALUES('00000000-0000-4000-8000-000000000088','Preserved','facility','fixture','active',12.25,-45.5,'2024-01-01T00:00:00Z','2024-01-02T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	before := entitySnapshot(t, db)
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db); len(got) != 1 || got[0] != before[0] {
		t.Fatalf("entity changed: before=%v after=%v", before, got)
	}
	if got := ledgerVersions(t, db); len(got) != 6 || got[1] != 2 || got[2] != 3 || got[3] != 4 || got[4] != 5 || got[5] != 6 {
		t.Fatalf("ledger=%v", got)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db)[0]; got != before[0] {
		t.Fatalf("reopen/rerun changed entity: %s", got)
	}
}

func TestV23MetadataAndActivitySurviveSearchMigration(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "v23-search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	v1, err := fs.ReadFile(migrations, "migrations/001_create_entities.sql")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := fs.ReadFile(migrations, "migrations/002_advanced_entity_management.sql")
	if err != nil {
		t.Fatal(err)
	}
	v23 := fstest.MapFS{
		"migrations/001_create_entities.sql":            &fstest.MapFile{Data: v1},
		"migrations/002_advanced_entity_management.sql": &fstest.MapFile{Data: v2},
	}
	if err := MigrateFS(ctx, db, v23); err != nil {
		t.Fatalf("prepare V2.3 schema: %v", err)
	}
	const entityID = "00000000-0000-4000-8000-000000000077"
	_, err = db.Exec(`INSERT INTO entities VALUES(?,?,?,?,?,?,?,?,?)`, entityID, "Searchable V2.3 asset", "vehicle", "legacy searchable description", "active", 1.25, -2.5, "2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO attribute_definitions(id,entity_type,key,label,value_type,required,archived,created_at,updated_at) VALUES('definition-1','vehicle','serial','Serial','string',0,0,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO entity_attribute_values(entity_id,definition_id,text_value) VALUES(?, 'definition-1','A-7')`, entityID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tags(id,name,normalized_name,created_at) VALUES('tag-1','Fleet','fleet','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO entity_tags(entity_id,tag_id) VALUES(?, 'tag-1')`, entityID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO categories(id,name,normalized_name,created_at) VALUES('category-1','Operations','operations','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO entity_categories(entity_id,category_id) VALUES(?, 'category-1')`, entityID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO activity_events(id,entity_id,event_type,occurred_at,actor_kind,payload_version,payload) VALUES('event-1',?,'entity.created','2024-01-01T00:00:00.000000000Z','system',1,'{}')`, entityID)
	if err != nil {
		t.Fatal(err)
	}
	before := entitySnapshot(t, db)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("apply V2.4 migration: %v", err)
	}
	if after := entitySnapshot(t, db); len(after) != 1 || after[0] != before[0] {
		t.Fatalf("entity changed across search migration: before=%v after=%v", before, after)
	}
	for table, want := range map[string]int{"entity_attribute_values": 1, "entity_tags": 1, "entity_categories": 1, "activity_events": 1} {
		var got int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&got); err != nil || got != want {
			t.Fatalf("%s count=%d want=%d err=%v", table, got, want, err)
		}
	}
	var matches int
	if err := db.QueryRow(`SELECT count(*) FROM entity_search WHERE entity_search MATCH ?`, `"Searchable V2.3"`).Scan(&matches); err != nil || matches != 1 {
		t.Fatalf("V2.3 data not indexed: count=%d err=%v", matches, err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if got := ledgerVersions(t, db); len(got) != 6 || got[2] != 3 || got[3] != 4 || got[4] != 5 || got[5] != 6 {
		t.Fatalf("migration ledger=%v", got)
	}
}

func TestFailedSearchMigrationRollsBackVirtualTableTriggersIndexesAndLedger(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "search-migration-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	v1, err := fs.ReadFile(migrations, "migrations/001_create_entities.sql")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := fs.ReadFile(migrations, "migrations/002_advanced_entity_management.sql")
	if err != nil {
		t.Fatal(err)
	}
	v3, err := fs.ReadFile(migrations, "migrations/003_entity_search_and_pagination.sql")
	if err != nil {
		t.Fatal(err)
	}
	baseline := fstest.MapFS{
		"migrations/001_create_entities.sql":            &fstest.MapFile{Data: v1},
		"migrations/002_advanced_entity_management.sql": &fstest.MapFile{Data: v2},
	}
	if err := MigrateFS(ctx, db, baseline); err != nil {
		t.Fatal(err)
	}
	broken := append(append([]byte(nil), v3...), []byte("\nCREATE TABLE idx_entities_type_created_at_id(id INTEGER);\n")...)
	withBrokenV3 := fstest.MapFS{
		"migrations/001_create_entities.sql":              &fstest.MapFile{Data: v1},
		"migrations/002_advanced_entity_management.sql":   &fstest.MapFile{Data: v2},
		"migrations/003_entity_search_and_pagination.sql": &fstest.MapFile{Data: broken},
	}
	if err := MigrateFS(ctx, db, withBrokenV3); err == nil {
		t.Fatal("expected injected migration failure")
	}
	for _, name := range []string{"entity_search", "entities_search_ai", "entities_search_ad", "entities_search_au", "idx_entities_type_created_at_id", "idx_entities_status_created_at_id"} {
		var found int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, name).Scan(&found); err != nil {
			t.Fatal(err)
		}
		if found != 0 {
			t.Errorf("failed migration left schema object %q", name)
		}
	}
	if got := ledgerVersions(t, db); len(got) != 2 || got[1] != 2 {
		t.Fatalf("failed migration changed ledger: %v", got)
	}
}

func TestEntityCreateRollsBackWhenActivityInsertFails(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rollback.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.db.Exec(`CREATE TRIGGER reject_events BEFORE INSERT ON activity_events BEGIN SELECT RAISE(ABORT,'injected event failure'); END`); err != nil {
		t.Fatal(err)
	}
	item := entity.Entity{ID: "00000000-0000-4000-8000-000000000090", Name: "Rollback", Type: "facility", Status: "active", Latitude: 1, Longitude: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.Create(ctx, item); err == nil {
		t.Fatal("expected activity insert failure")
	}
	if _, err = store.Get(ctx, item.ID); err != ErrNotFound {
		t.Fatalf("source mutation was not rolled back: %v", err)
	}
}

func TestMetadataTransactionsRollbackWhenActivityInsertFails(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "metadata-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := "00000000-0000-4000-8000-000000000091"
	now := time.Now().UTC()
	item := entity.Entity{ID: id, Name: "Asset", Type: "vehicle", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err = store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	d := Definition{EntityType: "vehicle", Key: "serial", Label: "Serial", ValueType: "string"}
	if err = store.SaveDefinition(ctx, d, true); err != nil {
		t.Fatal(err)
	}
	defs, err := store.Definitions(ctx, "vehicle")
	if err != nil || len(defs) != 1 {
		t.Fatalf("definitions=%v err=%v", defs, err)
	}
	if err = store.ReplaceAttributes(ctx, id, map[string]any{"serial": "old"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_attribute_events BEFORE INSERT ON activity_events WHEN NEW.event_type='entity.attributes.updated' BEGIN SELECT RAISE(ABORT,'injected attribute event failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = store.ReplaceAttributes(ctx, id, map[string]any{"serial": "new"}); err == nil {
		t.Fatal("expected attribute event failure")
	}
	_, values, _, err := store.AttributeValues(ctx, id)
	if err != nil || values["serial"] != "old" {
		t.Fatalf("attribute mutation escaped rollback: values=%v err=%v", values, err)
	}
	cat, err := store.CreateCatalog(ctx, "tags", "Operations")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_tag_events BEFORE INSERT ON activity_events WHEN NEW.event_type='entity.tags.updated' BEGIN SELECT RAISE(ABORT,'injected tag event failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = store.ReplaceCatalog(ctx, id, "tags", []string{cat.ID}); err == nil {
		t.Fatal("expected assignment event failure")
	}
	assigned, err := store.EntityCatalog(ctx, id, "tags")
	if err != nil || len(assigned) != 0 {
		t.Fatalf("tag assignment escaped rollback: %v %v", assigned, err)
	}
}

func TestCategoryAssignmentRollsBackWhenActivityInsertFails(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "category-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := "00000000-0000-4000-8000-000000000092"
	now := time.Now().UTC()
	item := entity.Entity{ID: id, Name: "Asset", Type: "facility", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err = store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	cat, err := store.CreateCatalog(ctx, "categories", "Field Operations")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_category_events BEFORE INSERT ON activity_events WHEN NEW.event_type='entity.categories.updated' BEGIN SELECT RAISE(ABORT,'injected category event failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = store.ReplaceCatalog(ctx, id, "categories", []string{cat.ID}); err == nil {
		t.Fatal("expected category assignment event failure")
	}
	assigned, err := store.EntityCatalog(ctx, id, "categories")
	if err != nil || len(assigned) != 0 {
		t.Fatalf("category assignment escaped rollback: %v %v", assigned, err)
	}
}

func TestActivityCursorStableForTimestampTies(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cursor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, id := range []string{"00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"} {
		if _, err = store.db.Exec(`INSERT INTO activity_events(id,entity_id,event_type,occurred_at,actor_kind,actor_id,payload_version,payload) VALUES(?,?,'entity.updated','2025-01-01T00:00:00.100000000Z','system',NULL,1,'{}')`, id, "00000000-0000-4000-8000-000000000099"); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.Activities(ctx, "00000000-0000-4000-8000-000000000099", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 2 || first.Data[0].ID != "00000000-0000-4000-8000-000000000003" || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second, err := store.Activities(ctx, "00000000-0000-4000-8000-000000000099", 2, first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Data) != 1 || second.Data[0].ID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("unexpected continuation: %+v", second)
	}
}
