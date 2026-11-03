package repository

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/geofence"
)

// BenchmarkGeofenceSynchronousUpdateMatrix measures the existing synchronous transaction
// over deterministic circle, polygon, and mixed definition sets on an isolated database.
func BenchmarkGeofenceSynchronousUpdateMatrix(b *testing.B) {
	for _, shape := range []string{"circles", "polygons_64", "mixed"} {
		for _, count := range []int{1, 10, 50, 100} {
			b.Run(fmt.Sprintf("%s/zones_%d", shape, count), func(b *testing.B) {
				ctx := context.Background()
				store, err := Open(ctx, filepath.Join(b.TempDir(), "matrix.db"))
				if err != nil {
					b.Fatal(err)
				}
				defer store.Close()
				at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				for i := 0; i < count; i++ {
					geometry := benchGeometry(shape, i)
					id := fmt.Sprintf("20000000-0000-4000-8000-%012d", i+1)
					f := geofence.Definition{ID: id, Name: fmt.Sprintf("Bench zone %03d", i), Enabled: true, Geometry: geometry, CreatedAt: at, UpdatedAt: at}
					if err = store.CreateGeofence(ctx, f); err != nil {
						b.Fatal(err)
					}
				}
				item := entity.Entity{ID: "10000000-0000-4000-8000-000000000001", Name: "Synchronous benchmark", Type: "facility", Status: "active", Latitude: -6.2, Longitude: 106.8, CreatedAt: at, UpdatedAt: at}
				if err = store.Create(ctx, item); err != nil {
					b.Fatal(err)
				}
				step := 0
				run := func() error {
					step++
					item.Longitude = 106.8 + float64(step%2)*0.000001
					item.UpdatedAt = at.Add(time.Duration(step) * time.Nanosecond)
					return store.Update(ctx, item)
				}
				// Capture a small un-timed sample for readable tail latency alongside Go's timed allocation data.
				samples := make([]int64, 50)
				for i := range samples {
					start := time.Now()
					if err = run(); err != nil {
						b.Fatal(err)
					}
					samples[i] = time.Since(start).Nanoseconds()
				}
				sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
				p50, p95 := float64(samples[24])/1000, float64(samples[47])/1000
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err = run(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(p50, "p50_us")
				b.ReportMetric(p95, "p95_us")
				b.ReportMetric(float64(count), "enabled_zones")
			})
		}
	}
}

func benchGeometry(shape string, index int) geofence.Geometry {
	if shape == "circles" || shape == "mixed" && index%2 == 0 {
		center := geofence.Position{106.8, -6.2}
		radius := 250.0
		return geofence.Geometry{Type: "Circle", Center: &center, RadiusM: &radius}
	}
	const vertices = 64
	ring := make([]geofence.Position, vertices+1)
	for i := 0; i < vertices; i++ {
		angle := 2 * math.Pi * float64(i) / vertices
		ring[i] = geofence.Position{106.8 + 0.002*math.Cos(angle), -6.2 + 0.002*math.Sin(angle)}
	}
	ring[vertices] = ring[0]
	return geofence.Geometry{Type: "Polygon", Coordinates: [][]geofence.Position{ring}}
}
