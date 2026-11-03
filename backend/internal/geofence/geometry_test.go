package geofence

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func parseTestGeometry(t *testing.T, raw string) Geometry {
	t.Helper()
	g, err := ParseGeometry(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("ParseGeometry: %v", err)
	}
	return g
}

func TestCircleContainmentAndBoundary(t *testing.T) {
	g := parseTestGeometry(t, `{"type":"Circle","center":[0,0],"radius_m":1000}`)
	for _, test := range []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{"center", 0, 0, true},
		{"inside", 0, 0.005, true},
		{"outside", 0, 0.02, false},
		{"invalid latitude", 91, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := g.Contains(test.lat, test.lon); got != test.want {
				t.Fatalf("Contains(%v,%v)=%v want %v", test.lat, test.lon, got, test.want)
			}
		})
	}

	boundary := haversineMeters(Position{0, 0}, Position{0, 1})
	g.RadiusM = &boundary
	if !g.Contains(1, 0) {
		t.Fatal("exact circle boundary must be inside")
	}
}

func TestCircleWrapsAcrossAntimeridianAndRejectsInvalidRadius(t *testing.T) {
	g := parseTestGeometry(t, `{"type":"Circle","center":[179.9,0],"radius_m":30000}`)
	if !g.Contains(0, -179.9) {
		t.Fatal("circle containment should use geodesic distance across antimeridian")
	}
	for _, raw := range []string{
		`{"type":"Circle","center":[0,0],"radius_m":0}`,
		`{"type":"Circle","center":[0,0],"radius_m":-1}`,
		fmt.Sprintf(`{"type":"Circle","center":[0,0],"radius_m":%d}`, MaxCircleRadiusM+1),
		`{"type":"Circle","center":[181,0],"radius_m":1}`,
		`{"type":"Circle","center":[0],"radius_m":1}`,
		`{"type":"Circle","center":[0,0],"radius_m":1,"coordinates":[]}`,
	} {
		if _, err := ParseGeometry(json.RawMessage(raw)); err == nil {
			t.Errorf("ParseGeometry(%s) unexpectedly succeeded", raw)
		}
	}
}

func TestPolygonContainsInsideOutsideBoundaryAndHighLatitude(t *testing.T) {
	g := parseTestGeometry(t, `{"type":"Polygon","coordinates":[[[-1,-1],[1,-1],[1,1],[-1,1],[-1,-1]]]}`)
	for _, test := range []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{"inside", 0, 0, true},
		{"outside", 2, 0, false},
		{"boundary", 0, 1, true},
		{"one centimetre outside boundary", 0.5, 1 + 0.01/111195.08, false},
	} {
		if got := g.Contains(test.lat, test.lon); got != test.want {
			t.Errorf("%s: Contains=%v want %v", test.name, got, test.want)
		}
	}

	high := parseTestGeometry(t, `{"type":"Polygon","coordinates":[[[-10,84],[10,84],[10,86],[-10,86],[-10,84]]]}`)
	if !high.Contains(85, 0) || high.Contains(80, 0) {
		t.Fatal("high-latitude spherical containment failed")
	}
}

func TestPolygonNormalizesWinding(t *testing.T) {
	g := parseTestGeometry(t, `{"type":"Polygon","coordinates":[[[-1,-1],[-1,1],[1,1],[1,-1],[-1,-1]]]}`)
	if planarSignedArea(g.Coordinates[0][:len(g.Coordinates[0])-1]) <= 0 {
		t.Fatal("exterior ring should be normalized counter-clockwise")
	}
	if !g.Contains(0, 0) {
		t.Fatal("reversed input ring should retain its intended area")
	}
}

func TestPolygonRejectsUnsupportedOrInvalidRings(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"unclosed", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,1]]]}`},
		{"too few", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`},
		{"degenerate", `{"type":"Polygon","coordinates":[[[0,0],[1,1],[2,2],[0,0]]]}`},
		{"self intersecting", `{"type":"Polygon","coordinates":[[[0,0],[2,2],[0,2],[2,0],[0,0]]]}`},
		{"duplicate vertex", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[1,0],[0,0]]]}`},
		{"adjacent edge backtrack", `{"type":"Polygon","coordinates":[[[0,0],[2,0],[1,0],[1,1],[0,0]]]}`},
		{"out of range", `{"type":"Polygon","coordinates":[[[0,0],[181,0],[1,1],[0,0]]]}`},
		{"hole unsupported", `{"type":"Polygon","coordinates":[[[0,0],[2,0],[2,2],[0,2],[0,0]],[[0.5,0.5],[1,0.5],[1,1],[0.5,0.5]]]}`},
		{"antimeridian edge", `{"type":"Polygon","coordinates":[[[179,0],[-179,0],[-179,1],[179,1],[179,0]]]}`},
		{"antimeridian span", `{"type":"Polygon","coordinates":[[[-100,0],[100,0],[100,1],[-100,1],[-100,0]]]}`},
		{"unsupported type", `{"type":"MultiPolygon","coordinates":[]}`},
		{"unknown field", `{"type":"Polygon","coordinates":[],"extra":true}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseGeometry(json.RawMessage(test.raw)); err == nil {
				t.Fatalf("ParseGeometry(%s) unexpectedly succeeded", test.raw)
			}
		})
	}
}

func TestPolygonVertexLimit(t *testing.T) {
	positions := make([]string, 0, MaxPolygonVertices+2)
	for i := 0; i <= MaxPolygonVertices+1; i++ {
		angle := 2 * math.Pi * float64(i) / float64(MaxPolygonVertices+1)
		positions = append(positions, fmt.Sprintf("[%.8f,%.8f]", 10*math.Cos(angle), 10*math.Sin(angle)))
	}
	raw := `{"type":"Polygon","coordinates":[[` + strings.Join(positions, ",") + `]]}`
	if _, err := ParseGeometry(json.RawMessage(raw)); err == nil {
		t.Fatal("ring exceeding vertex limit was accepted")
	}
}

func TestPolygonRejectsGeometryWithWrongCoordinateOrderShape(t *testing.T) {
	for _, raw := range []string{
		`{"type":"Polygon","coordinates":[[[0,0,4],[1,0,4],[0,1,4],[0,0,4]]]}`,
		`{"type":"Polygon","coordinates":[[["NaN",0],[1,0],[0,1],["NaN",0]]]}`,
	} {
		if _, err := ParseGeometry(json.RawMessage(raw)); err == nil {
			t.Errorf("invalid coordinate shape accepted: %s", raw)
		}
	}
}
