package repository

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/geofence"
)

func BenchmarkCoordinateUpdateWith100PolygonGeofences(b *testing.B) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(b.TempDir(), "membership-bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	const zoneCount, vertexCount = 100, 256
	base := time.Now().UTC()
	ring := make([]geofence.Position, vertexCount+1)
	for i := 0; i < vertexCount; i++ {
		angle := 2 * math.Pi * float64(i) / vertexCount
		ring[i] = geofence.Position{106.8 + 0.01*math.Cos(angle), -6.2 + 0.01*math.Sin(angle)}
	}
	ring[vertexCount] = ring[0]
	for i := 0; i < zoneCount; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		definition := geofence.Definition{ID: id, Name: fmt.Sprintf("Zone %03d", i), Enabled: true, Geometry: geofence.Geometry{Type: "Polygon", Coordinates: [][]geofence.Position{ring}}, CreatedAt: base, UpdatedAt: base}
		if err := store.CreateGeofence(ctx, definition); err != nil {
			b.Fatal(err)
		}
	}
	item := entity.Entity{ID: "10000000-0000-4000-8000-000000000001", Name: "Bench", Type: "facility", Status: "active", Latitude: -6.2, Longitude: 106.8, CreatedAt: base, UpdatedAt: base}
	if err := store.Create(ctx, item); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item.Longitude = 106.8 + float64((i%2)*2-1)*0.000001
		item.UpdatedAt = base.Add(time.Duration(i+1) * time.Nanosecond)
		if err := store.Update(ctx, item); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(zoneCount), "geofences/update")
	b.ReportMetric(float64(vertexCount), "vertices/geofence")
}

func BenchmarkGeofenceEventHistoryPageAt2000Events(b *testing.B) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(b.TempDir(), "events-bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	fence := circleDefinition("20000000-0000-4000-8000-000000000001", "History", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		b.Fatal(err)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2000; i++ {
		id := fmt.Sprintf("30000000-0000-4000-8000-%012d", i+1)
		if _, err := tx.ExecContext(ctx, `INSERT INTO geofence_events(id,entity_id,entity_name,geofence_id,geofence_name,event_type,latitude,longitude,occurred_at,source) VALUES(?,?,?,?,?,'enter',-6.2,106.8,?,'coordinate_update')`, id, "40000000-0000-4000-8000-000000000001", "Bench", fence.ID, fence.Name, formatStoredTimestamp(base.Add(time.Duration(i)*time.Second))); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, MaxGeofenceEventPageSize, ""); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(2000, "events/fixture")
	b.ReportMetric(float64(MaxGeofenceEventPageSize), "page_limit")
}
