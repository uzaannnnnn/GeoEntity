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
	if got := ledgerVersions(t, db); len(got) != 2 || got[1] != 2 {
		t.Fatalf("ledger=%v", got)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db)[0]; got != before[0] {
		t.Fatalf("reopen/rerun changed entity: %s", got)
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
