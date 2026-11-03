package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkEntityQueries10K(b *testing.B) {
	store, err := Open(context.Background(), filepath.Join(b.TempDir(), "query-benchmark.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	created := "2026-11-03T00:00:00Z"
	tx, err := store.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES(?,?,?,?,?,0,0,?,?)`)
	if err != nil {
		tx.Rollback()
		b.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("Asset record %05d", i)
		description := "routine equipment record"
		if i%100 == 0 {
			description = "needle maintenance record"
		}
		status, typ := "active", "facility"
		if i%2 == 1 {
			status, typ = "inactive", "vehicle"
		}
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		if _, err := stmt.Exec(id, name, typ, description, status, created, created); err != nil {
			stmt.Close()
			tx.Rollback()
			b.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		tx.Rollback()
		b.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	deepCursor, err := encodeEntityCursor(entityCursor{
		Version: 1, CreatedAt: created,
		ID:         fmt.Sprintf("00000000-0000-4000-8000-%012d", 9000),
		FilterHash: entityFilterHash(EntityFilters{}),
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(10000, "entities")
	benchmarks := []struct {
		name  string
		query EntityQuery
	}{
		{name: "short_fallback", query: EntityQuery{EntityFilters: EntityFilters{Q: "As"}, Limit: 50}},
		{name: "three_character_trigram", query: EntityQuery{EntityFilters: EntityFilters{Q: "set"}, Limit: 50}},
		{name: "long_trigram", query: EntityQuery{EntityFilters: EntityFilters{Q: "record 099"}, Limit: 50}},
		{name: "combined_filters", query: EntityQuery{EntityFilters: EntityFilters{Q: "needle", Type: "facility", Status: "active"}, Limit: 50}},
		{name: "first_page", query: EntityQuery{Limit: 50}},
		{name: "deep_cursor", query: EntityQuery{Limit: 50, Cursor: deepCursor}},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := store.QueryEntities(context.Background(), benchmark.query); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	b.Run("map_viewport", func(b *testing.B) {
		bounds := MapBounds{West: -1, South: -1, East: 1, North: 1}
		filters := EntityFilters{}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// This returns ErrMapResultLimit because 10,000 entities match (> 5,000 limit)
			_, _ = store.MapEntities(context.Background(), filters, bounds)
		}
	})

	b.Run("map_extent", func(b *testing.B) {
		filters := EntityFilters{}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := store.MapEntityExtent(context.Background(), filters); err != nil {
				b.Fatal(err)
			}
		}
	})
}
