package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/entity"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func openQueryStore(t *testing.T) *SQLite {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "query.db"))
	if err != nil {
		t.Fatalf("open query store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close query store: %v", err)
		}
	})
	return store
}

func insertQueryEntity(t *testing.T, store *SQLite, id, name, description, typ, status string, lat, lon float64, created string) {
	t.Helper()
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		t.Fatal(err)
	}
	item := entity.Entity{ID: id, Name: name, Type: typ, Description: description, Status: status, Latitude: lat, Longitude: lon, CreatedAt: createdAt, UpdatedAt: createdAt}
	if err := store.Create(context.Background(), item); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func TestFTS5TrigramCapabilityAndMigration(t *testing.T) {
	store := openQueryStore(t)
	var sqliteVersion string
	if err := store.db.QueryRow(`SELECT sqlite_version()`).Scan(&sqliteVersion); err != nil {
		t.Fatal(err)
	}
	t.Logf("SQLite runtime version: %s (modernc.org/sqlite v1.61.1)", sqliteVersion)
	var tableCount, triggerCount, indexCount int
	if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='entity_search'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name LIKE 'entities_search_%'`).Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name IN ('idx_entities_type_created_at_id','idx_entities_status_created_at_id')`).Scan(&indexCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 1 || triggerCount != 3 || indexCount != 2 {
		t.Fatalf("SQLite %s migration structures: FTS=%d triggers=%d indexes=%d", sqliteVersion, tableCount, triggerCount, indexCount)
	}
	if _, err := store.db.Exec(`INSERT INTO entity_search(entity_search) VALUES('integrity-check')`); err != nil {
		t.Fatalf("FTS5 trigram integrity check on SQLite %s: %v", sqliteVersion, err)
	}
	if _, err := store.db.Exec(`SELECT rowid FROM entity_search WHERE entity_search MATCH ?`, `"capability"`); err != nil {
		t.Fatalf("FTS5 trigram MATCH unavailable on SQLite %s: %v", sqliteVersion, err)
	}
}

func TestSearchSemanticsAndEntityMutationSynchronization(t *testing.T) {
	store := openQueryStore(t)
	created := "2026-11-03T00:00:00Z"
	ids := []string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000004",
	}
	insertQueryEntity(t, store, ids[0], `Truck Alpha`, `Depot "A" + OR`, "vehicle", "active", -6.2, 106.8, created)
	insertQueryEntity(t, store, ids[1], `TRUCK Beta`, `north gate`, "vehicle", "inactive", -6.3, 106.9, created)
	insertQueryEntity(t, store, ids[2], `Café Pump+Station`, `Zone_A (north)`, "facility", "maintenance", -6.4, 106.7, created)
	insertQueryEntity(t, store, ids[3], `Quote " OR NOT`, `SQL ' ; DROP TABLE entities;--`, "other", "active", -6.5, 106.6, created)

	tests := []struct {
		name    string
		filters EntityFilters
		wantIDs []string
	}{
		{name: "empty query", wantIDs: ids},
		{name: "short ascii query", filters: EntityFilters{Q: "  tr  "}, wantIDs: ids[:2]},
		{name: "normalized whitespace", filters: EntityFilters{Q: "  Truck\t  Alpha "}, wantIDs: ids[:1]},
		{name: "trigram substring case-insensitive", filters: EntityFilters{Q: "rUcK"}, wantIDs: ids[:2]},
		{name: "quoted punctuation", filters: EntityFilters{Q: `"A" + OR`}, wantIDs: ids[:1]},
		{name: "unicode trigram", filters: EntityFilters{Q: "afé"}, wantIDs: ids[2:3]},
		{name: "special FTS punctuation literal", filters: EntityFilters{Q: "Zone_A (north)"}, wantIDs: ids[2:3]},
		{name: "sql-looking input is literal", filters: EntityFilters{Q: `DROP TABLE entities;--`}, wantIDs: ids[3:4]},
		{name: "fts operators are literal", filters: EntityFilters{Q: "OR NOT"}, wantIDs: ids[3:4]},
		{name: "colon is literal not column filter", filters: EntityFilters{Q: "name:Truck"}, wantIDs: []string{}},
		{name: "asterisk wildcard is literal", filters: EntityFilters{Q: "Truck*"}, wantIDs: []string{}},
		{name: "single quote is literal", filters: EntityFilters{Q: "SQL ' ;"}, wantIDs: ids[3:4]},
		{name: "percent and underscore are literal", filters: EntityFilters{Q: "Zone_A"}, wantIDs: ids[2:3]},
		{name: "short single character fallback", filters: EntityFilters{Q: "C"}, wantIDs: ids[:3]},
		{name: "short quote character fallback", filters: EntityFilters{Q: `"`}, wantIDs: []string{ids[0], ids[3]}},
		{name: "type filter", filters: EntityFilters{Type: "vehicle"}, wantIDs: ids[:2]},
		{name: "status filter", filters: EntityFilters{Status: "active"}, wantIDs: []string{ids[0], ids[3]}},
		{name: "combined search and type/status", filters: EntityFilters{Q: "truck", Type: "vehicle", Status: "inactive"}, wantIDs: ids[1:2]},
		{name: "no match", filters: EntityFilters{Q: "nothing-matches"}, wantIDs: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: test.filters, Limit: 100})
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			got := make([]string, len(page.Data))
			for i, item := range page.Data {
				got[i] = item.ID
			}
			if strings.Join(got, ",") != strings.Join(test.wantIDs, ",") {
				t.Fatalf("IDs=%v want %v", got, test.wantIDs)
			}
		})
	}

	// The trigger removes the old search document and inserts the updated one.
	updated, err := store.Get(context.Background(), ids[0])
	if err != nil {
		t.Fatal(err)
	}
	updated.Name = "Moved Vehicle"
	updated.Description = "updated narrative"
	updated.Type = "facility"
	updated.Status = "inactive"
	updated.UpdatedAt = updated.UpdatedAt.Add(time.Second)
	if err := store.Update(context.Background(), updated); err != nil {
		t.Fatal(err)
	}
	oldPage, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "Truck Alpha"}})
	if err != nil || len(oldPage.Data) != 0 {
		t.Fatalf("old text remained searchable: page=%v err=%v", oldPage.Data, err)
	}
	newPage, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "Moved Vehicle"}})
	if err != nil || len(newPage.Data) != 1 || newPage.Data[0].ID != ids[0] {
		t.Fatalf("updated text not searchable: page=%v err=%v", newPage.Data, err)
	}
	updatedFilter, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Type: "facility", Status: "inactive"}})
	if err != nil || len(updatedFilter.Data) != 1 || updatedFilter.Data[0].ID != ids[0] {
		t.Fatalf("updated type/status filter was not reflected: page=%v err=%v", updatedFilter.Data, err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER reject_query_update BEFORE INSERT ON activity_events WHEN NEW.event_type='entity.updated' BEGIN SELECT RAISE(ABORT,'injected query update failure'); END`); err != nil {
		t.Fatal(err)
	}
	failedUpdate := updated
	failedUpdate.Name = "Failed update phrase"
	failedUpdate.Description = "failed transaction"
	if err := store.Update(context.Background(), failedUpdate); err == nil {
		t.Fatal("expected injected update failure")
	}
	if _, err := store.db.Exec(`DROP TRIGGER reject_query_update`); err != nil {
		t.Fatal(err)
	}
	rollbackPage, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "Moved Vehicle"}})
	if err != nil || len(rollbackPage.Data) != 1 {
		t.Fatalf("failed update removed old FTS document: page=%v err=%v", rollbackPage.Data, err)
	}
	failedPage, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "Failed update phrase"}})
	if err != nil || len(failedPage.Data) != 0 {
		t.Fatalf("failed update FTS document leaked: page=%v err=%v", failedPage.Data, err)
	}
	if err := store.Delete(context.Background(), ids[0]); err != nil {
		t.Fatal(err)
	}
	deletedPage, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "Moved Vehicle"}})
	if err != nil || len(deletedPage.Data) != 0 {
		t.Fatalf("deleted entity remained searchable: page=%v err=%v", deletedPage.Data, err)
	}
}

func TestEntityQueryPaginationCursorAndConcurrentChanges(t *testing.T) {
	store := openQueryStore(t)
	created := "2026-11-03T00:00:00Z"
	ids := []string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000004",
		"00000000-0000-4000-8000-000000000005",
		"00000000-0000-4000-8000-000000000006",
	}
	for i, id := range ids[:5] {
		insertQueryEntity(t, store, id, "Asset", "same timestamp", "facility", "active", float64(i), float64(i), created)
	}
	first, err := store.QueryEntities(context.Background(), EntityQuery{Limit: 2})
	if err != nil || len(first.Data) != 2 || first.Meta.NextCursor == "" || first.Meta.MatchingCount != 5 {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	if first.Data[0].ID != ids[0] || first.Data[1].ID != ids[1] {
		t.Fatalf("unstable tie order: %s %s", first.Data[0].ID, first.Data[1].ID)
	}
	if first.Meta.StatusCounts["active"] != 5 || first.Meta.StatusCounts["inactive"] != 0 {
		t.Fatalf("unexpected status counts: %v", first.Meta.StatusCounts)
	}
	second, err := store.QueryEntities(context.Background(), EntityQuery{Limit: 2, Cursor: first.Meta.NextCursor})
	if err != nil || len(second.Data) != 2 || second.Meta.NextCursor == "" {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
	if second.Data[0].ID != ids[2] || second.Data[1].ID != ids[3] {
		t.Fatalf("wrong second page order: %s %s", second.Data[0].ID, second.Data[1].ID)
	}
	if _, err := store.QueryEntities(context.Background(), EntityQuery{Limit: 2, Cursor: first.Meta.NextCursor, EntityFilters: EntityFilters{Type: "vehicle"}}); err != ErrInvalidCursor {
		t.Fatalf("filter-mismatched cursor error=%v", err)
	}
	for _, bad := range []string{"!", "e30", strings.Repeat("a", maxCursorLength+1)} {
		if _, err := store.QueryEntities(context.Background(), EntityQuery{Cursor: bad}); err != ErrInvalidCursor {
			t.Errorf("cursor %q error=%v", bad, err)
		}
	}
	if _, err := store.QueryEntities(context.Background(), EntityQuery{Limit: MaxEntityPageSize + 1}); err != ErrInvalidQuery {
		t.Fatalf("excessive limit error=%v", err)
	}
	if _, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Type: "sensor"}}); err != ErrInvalidQuery {
		t.Fatalf("invalid type error=%v", err)
	}

	// A newly inserted row with a key after the cursor appears on the next page.
	insertQueryEntity(t, store, ids[5], "Later asset", "new", "facility", "active", 5, 5, created)
	third, err := store.QueryEntities(context.Background(), EntityQuery{Limit: 2, Cursor: second.Meta.NextCursor})
	if err != nil || len(third.Data) != 2 || third.Data[0].ID != ids[4] || third.Data[1].ID != ids[5] || third.Meta.NextCursor != "" {
		t.Fatalf("concurrent insertion page=%+v err=%v", third, err)
	}
	if err := store.Delete(context.Background(), ids[4]); err != nil {
		t.Fatal(err)
	}
	deletedCursorPage, err := store.QueryEntities(context.Background(), EntityQuery{Limit: 2, Cursor: second.Meta.NextCursor})
	if err != nil || len(deletedCursorPage.Data) != 1 || deletedCursorPage.Data[0].ID != ids[5] {
		t.Fatalf("concurrent deletion page=%+v err=%v", deletedCursorPage, err)
	}
	empty, err := store.QueryEntities(context.Background(), EntityQuery{EntityFilters: EntityFilters{Q: "no such entity"}})
	if err != nil || empty.Data == nil || len(empty.Data) != 0 || empty.Meta.NextCursor != "" {
		t.Fatalf("empty page=%+v err=%v", empty, err)
	}
}

func TestNormalizeEntityFiltersAndCursorEncoding(t *testing.T) {
	filters, err := NormalizeEntityFilters(EntityFilters{Q: "  one\t two\nthree  "})
	if err != nil || filters.Q != "one two three" {
		t.Fatalf("normalized filters=%+v err=%v", filters, err)
	}
	if _, err := NormalizeEntityFilters(EntityFilters{Q: strings.Repeat("é", maxQueryLength+1)}); err != ErrInvalidQuery {
		t.Fatalf("long rune query error=%v", err)
	}
	if _, err := NormalizeEntityFilters(EntityFilters{Q: string([]byte{0xff})}); err != ErrInvalidQuery {
		t.Fatalf("invalid UTF-8 query error=%v", err)
	}
	cursor := entityCursor{Version: 1, CreatedAt: "2026-11-03T00:00:00Z", ID: uuid.NewString(), FilterHash: entityFilterHash(filters)}
	encoded, err := encodeEntityCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeEntityCursor(encoded)
	if err != nil || *decoded != cursor {
		t.Fatalf("decoded cursor=%+v err=%v", decoded, err)
	}
	// Extra JSON is rejected rather than being silently ignored.
	jsonBytes, _ := json.Marshal(cursor)
	var withExtra map[string]any
	_ = json.Unmarshal(jsonBytes, &withExtra)
	withExtra["internal"] = true
	extra, _ := json.Marshal(withExtra)
	if _, err := decodeEntityCursor(base64.RawURLEncoding.EncodeToString(extra)); err == nil {
		t.Fatal("cursor with unknown property was accepted")
	}
}

func TestMapQueryIndependentBoundsExtentAndAntimeridian(t *testing.T) {
	store := openQueryStore(t)
	created := "2026-11-03T00:00:00Z"
	ids := []string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003",
	}
	insertQueryEntity(t, store, ids[0], "West", "", "facility", "active", 1, 179, created)
	insertQueryEntity(t, store, ids[1], "East", "", "facility", "inactive", 2, -179, created)
	insertQueryEntity(t, store, ids[2], "Middle", "", "vehicle", "active", 3, 0, created)
	insertQueryEntity(t, store, "00000000-0000-4000-8000-000000000004", "SamePoint one", "", "other", "active", 4, 5, created)
	insertQueryEntity(t, store, "00000000-0000-4000-8000-000000000005", "SamePoint two", "", "other", "active", 4, 5, created)
	features, err := store.MapEntities(context.Background(), EntityFilters{}, MapBounds{West: 170, South: 0, East: -170, North: 3})
	if err != nil || len(features) != 2 {
		t.Fatalf("dateline map features=%+v err=%v", features, err)
	}
	filtered, err := store.MapEntities(context.Background(), EntityFilters{Type: "vehicle", Status: "active"}, MapBounds{West: -180, South: -90, East: 180, North: 90})
	if err != nil || len(filtered) != 1 || filtered[0].ID != ids[2] {
		t.Fatalf("filtered map features=%+v err=%v", filtered, err)
	}
	extent, err := store.MapEntityExtent(context.Background(), EntityFilters{})
	if err != nil || extent.Count != 5 || extent.Bounds == nil || extent.Bounds.West != -179 || extent.Bounds.East != 179 {
		t.Fatalf("extent=%+v err=%v", extent, err)
	}
	identical, err := store.MapEntityExtent(context.Background(), EntityFilters{Q: "SamePoint"})
	if err != nil || identical.Count != 2 || identical.Bounds == nil || identical.Bounds.West != 5 || identical.Bounds.East != 5 || identical.Bounds.South != 4 || identical.Bounds.North != 4 {
		t.Fatalf("identical-coordinate extent=%+v err=%v", identical, err)
	}
	empty, err := store.MapEntityExtent(context.Background(), EntityFilters{Q: "absent"})
	if err != nil || empty.Count != 0 || empty.Bounds != nil {
		t.Fatalf("empty extent=%+v err=%v", empty, err)
	}
	if _, err := store.MapEntities(context.Background(), EntityFilters{}, MapBounds{West: 1, South: 5, East: 2, North: 4}); err != ErrInvalidBounds {
		t.Fatalf("inverted latitude bounds error=%v", err)
	}

	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES(?, 'Dense','facility','','active',0,0,?,?)`)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	for i := 0; i <= MaxMapFeatures; i++ {
		if _, err := stmt.Exec(uuid.NewString(), created, created); err != nil {
			stmt.Close()
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MapEntities(context.Background(), EntityFilters{}, MapBounds{West: -1, South: -1, East: 1, North: 1}); err != ErrMapResultLimit {
		t.Fatalf("map overflow error=%v", err)
	}
}

func TestQueryPlansUseFTSAndPaginationIndexes(t *testing.T) {
	store := openQueryStore(t)
	queries := []struct {
		name string
		sql  string
		args []any
		want string
	}{
		{name: "FTS trigram", sql: `EXPLAIN QUERY PLAN SELECT e.id FROM entities e WHERE e.rowid IN (SELECT rowid FROM entity_search WHERE entity_search MATCH ?) ORDER BY e.created_at,e.id`, args: []any{`"asset"`}, want: "VIRTUAL TABLE INDEX"},
		{name: "unfiltered pagination", sql: `EXPLAIN QUERY PLAN SELECT id FROM entities ORDER BY created_at,id LIMIT ?`, args: []any{50}, want: "idx_entities_created_at"},
		{name: "type pagination", sql: `EXPLAIN QUERY PLAN SELECT id FROM entities WHERE type=? ORDER BY created_at,id LIMIT ?`, args: []any{"vehicle", 50}, want: "idx_entities_type_created_at_id"},
		{name: "status pagination", sql: `EXPLAIN QUERY PLAN SELECT id FROM entities WHERE status=? ORDER BY created_at,id LIMIT ?`, args: []any{"active", 50}, want: "idx_entities_status_created_at_id"},
		{name: "short query fallback", sql: `EXPLAIN QUERY PLAN SELECT id FROM entities WHERE (instr(lower(name),lower(?)) > 0 OR instr(lower(description),lower(?)) > 0) ORDER BY created_at,id LIMIT ?`, args: []any{"as", "as", 50}, want: "SCAN"},
	}
	for _, test := range queries {
		t.Run(test.name, func(t *testing.T) {
			rows, err := store.db.Query(test.sql, test.args...)
			if err != nil {
				t.Fatal(err)
			}
			var details []string
			for rows.Next() {
				var id, parent, notused int
				var detail string
				if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				details = append(details, detail)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(details, "\n"), test.want) {
				t.Fatalf("query plan did not include %q: %v", test.want, details)
			}
			t.Logf("query plan: %s", strings.Join(details, " | "))
		})
	}
}
