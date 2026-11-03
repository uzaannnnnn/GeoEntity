package geofence

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func BenchmarkParsePolygon256Vertices(b *testing.B) {
	positions := make([]string, 0, MaxPolygonVertices+1)
	for i := 0; i < MaxPolygonVertices; i++ {
		angle := 2 * math.Pi * float64(i) / float64(MaxPolygonVertices)
		positions = append(positions, fmt.Sprintf("[%.8f,%.8f]", 10*math.Cos(angle), 10*math.Sin(angle)))
	}
	positions = append(positions, positions[0])
	raw := json.RawMessage(`{"type":"Polygon","coordinates":[[` + strings.Join(positions, ",") + `]]}`)
	b.ReportAllocs()
	b.ReportMetric(MaxPolygonVertices, "vertices")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseGeometry(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContainsActivePolygonSets(b *testing.B) {
	positions := make([]string, 0, MaxPolygonVertices+1)
	for i := 0; i < MaxPolygonVertices; i++ {
		angle := 2 * math.Pi * float64(i) / float64(MaxPolygonVertices)
		positions = append(positions, fmt.Sprintf("[%.8f,%.8f]", 10*math.Cos(angle), 10*math.Sin(angle)))
	}
	positions = append(positions, positions[0])
	raw := json.RawMessage(`{"type":"Polygon","coordinates":[[` + strings.Join(positions, ",") + `]]}`)
	geometry, err := ParseGeometry(raw)
	if err != nil {
		b.Fatal(err)
	}
	for _, count := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("zones_%d", count), func(b *testing.B) {
			b.ReportMetric(float64(count), "active_zones")
			b.ReportMetric(MaxPolygonVertices, "vertices_per_zone")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for range count {
					if !geometry.Contains(0, 0) {
						b.Fatal("fixture center must be inside")
					}
				}
			}
		})
	}
}
