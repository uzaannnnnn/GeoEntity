package geofence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	MaxPolygonVertices = 256
	MaxCircleRadiusM   = 10_000_000
	earthRadiusMeters  = 6_371_008.8
	boundaryTolerance  = 0.01
)

var (
	ErrInvalidGeometry = errors.New("invalid geofence geometry")
	ErrOutsideBoundary = errors.New("polygon must not cross or span the antimeridian")
)

// Position follows RFC 7946 ordering: longitude, latitude.
type Position [2]float64

type Geometry struct {
	Type        string       `json:"type"`
	Center      *Position    `json:"center,omitempty"`
	RadiusM     *float64     `json:"radius_m,omitempty"`
	Coordinates [][]Position `json:"coordinates,omitempty"`
}

type geometryInput struct {
	Type        string          `json:"type"`
	Center      json.RawMessage `json:"center"`
	RadiusM     json.RawMessage `json:"radius_m"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func ParseGeometry(raw json.RawMessage) (Geometry, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var in geometryInput
	if err := decoder.Decode(&in); err != nil {
		return Geometry{}, fmt.Errorf("%w: malformed geometry", ErrInvalidGeometry)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Geometry{}, fmt.Errorf("%w: geometry must be one object", ErrInvalidGeometry)
	}

	switch in.Type {
	case "Circle":
		if in.Coordinates != nil || len(in.Center) == 0 || len(in.RadiusM) == 0 {
			return Geometry{}, fmt.Errorf("%w: Circle requires center [longitude,latitude] and radius_m only", ErrInvalidGeometry)
		}
		var rawCenter []float64
		if err := json.Unmarshal(in.Center, &rawCenter); err != nil {
			return Geometry{}, fmt.Errorf("%w: invalid circle center", ErrInvalidGeometry)
		}
		center, err := parsePosition(rawCenter)
		if err != nil {
			return Geometry{}, fmt.Errorf("%w: invalid circle center", ErrInvalidGeometry)
		}
		var radius float64
		if err := json.Unmarshal(in.RadiusM, &radius); err != nil {
			return Geometry{}, fmt.Errorf("%w: radius_m must be a number", ErrInvalidGeometry)
		}
		if math.IsNaN(radius) || math.IsInf(radius, 0) || radius <= 0 || radius > MaxCircleRadiusM {
			return Geometry{}, fmt.Errorf("%w: radius_m must be finite, greater than zero, and at most %d", ErrInvalidGeometry, MaxCircleRadiusM)
		}
		return Geometry{Type: "Circle", Center: &center, RadiusM: &radius}, nil
	case "Polygon":
		if len(in.Center) != 0 || len(in.RadiusM) != 0 || len(in.Coordinates) == 0 {
			return Geometry{}, fmt.Errorf("%w: Polygon requires exactly one exterior ring and no circle fields", ErrInvalidGeometry)
		}
		var coordinates [][][]float64
		if err := json.Unmarshal(in.Coordinates, &coordinates); err != nil || len(coordinates) != 1 {
			return Geometry{}, fmt.Errorf("%w: Polygon requires exactly one exterior ring", ErrInvalidGeometry)
		}
		ring, err := parseRing(coordinates[0])
		if err != nil {
			return Geometry{}, err
		}
		return Geometry{Type: "Polygon", Coordinates: [][]Position{ring}}, nil
	default:
		return Geometry{}, fmt.Errorf("%w: type must be Circle or Polygon", ErrInvalidGeometry)
	}
}

func parsePosition(raw []float64) (Position, error) {
	if len(raw) != 2 || !finite(raw[0]) || !finite(raw[1]) || raw[0] < -180 || raw[0] > 180 || raw[1] < -90 || raw[1] > 90 {
		return Position{}, ErrInvalidGeometry
	}
	return Position{raw[0], raw[1]}, nil
}

func parseRing(raw [][]float64) ([]Position, error) {
	if len(raw) < 4 || len(raw)-1 > MaxPolygonVertices {
		return nil, fmt.Errorf("%w: ring requires 4 to %d positions including closure", ErrInvalidGeometry, MaxPolygonVertices+1)
	}
	ring := make([]Position, len(raw))
	for i, position := range raw {
		point, err := parsePosition(position)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid position at index %d", ErrInvalidGeometry, i)
		}
		ring[i] = point
	}
	if ring[0] != ring[len(ring)-1] {
		return nil, fmt.Errorf("%w: linear ring must be closed", ErrInvalidGeometry)
	}
	ring = ring[:len(ring)-1]

	unique := make(map[Position]struct{}, len(ring))
	minLon, maxLon := ring[0][0], ring[0][0]
	for i, p := range ring {
		if _, exists := unique[p]; exists {
			return nil, fmt.Errorf("%w: ring contains a repeated vertex", ErrInvalidGeometry)
		}
		unique[p] = struct{}{}
		minLon = math.Min(minLon, p[0])
		maxLon = math.Max(maxLon, p[0])
		next := ring[(i+1)%len(ring)]
		if math.Abs(p[0]-next[0]) > 180 {
			return nil, ErrOutsideBoundary
		}
	}
	if len(unique) < 3 {
		return nil, fmt.Errorf("%w: ring requires at least three distinct vertices", ErrInvalidGeometry)
	}
	if maxLon-minLon >= 180 {
		return nil, ErrOutsideBoundary
	}
	if math.Abs(planarSignedArea(ring)) < 1e-12 {
		return nil, fmt.Errorf("%w: ring has zero area", ErrInvalidGeometry)
	}
	if hasAdjacentBacktrack(ring) {
		return nil, fmt.Errorf("%w: ring edges must not overlap or backtrack", ErrInvalidGeometry)
	}
	if hasSelfIntersection(ring) {
		return nil, fmt.Errorf("%w: ring must not self-intersect or touch itself", ErrInvalidGeometry)
	}
	// Normalize exterior rings to RFC 7946's counter-clockwise right-hand rule.
	if planarSignedArea(ring) < 0 {
		reverse(ring)
	}
	return append(ring, ring[0]), nil
}

func (g Geometry) Contains(latitude, longitude float64) bool {
	if !finite(latitude) || !finite(longitude) || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return false
	}
	point := Position{longitude, latitude}
	switch g.Type {
	case "Circle":
		if g.Center == nil || g.RadiusM == nil {
			return false
		}
		return haversineMeters(*g.Center, point) <= *g.RadiusM+boundaryTolerance
	case "Polygon":
		if len(g.Coordinates) != 1 {
			return false
		}
		return sphericalRingContains(g.Coordinates[0], point)
	default:
		return false
	}
}

func haversineMeters(a, b Position) float64 {
	lat1, lat2 := radians(a[1]), radians(b[1])
	dLat := lat2 - lat1
	dLon := radians(b[0] - a[0])
	sinLat, sinLon := math.Sin(dLat/2), math.Sin(dLon/2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon
	if h > 1 {
		h = 1
	}
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(h))
}

func sphericalRingContains(ring []Position, point Position) bool {
	p := vectorFor(point)
	var winding float64
	for i := 0; i < len(ring)-1; i++ {
		a, b := vectorFor(ring[i]), vectorFor(ring[i+1])
		if pointOnArc(p, a, b) {
			return true
		}
		ta := tangentAt(p, a)
		tb := tangentAt(p, b)
		if norm(ta) < 1e-14 || norm(tb) < 1e-14 {
			// The query is antipodal to a vertex. Such a point cannot be in a
			// ring whose longitude span is strictly less than 180 degrees.
			return false
		}
		ta, tb = unit(ta), unit(tb)
		winding += math.Atan2(dot(p, cross(ta, tb)), dot(ta, tb))
	}
	return math.Abs(winding) > math.Pi
}

type vector struct{ x, y, z float64 }

func vectorFor(p Position) vector {
	lon, lat := radians(p[0]), radians(p[1])
	cosLat := math.Cos(lat)
	return vector{cosLat * math.Cos(lon), cosLat * math.Sin(lon), math.Sin(lat)}
}
func radians(v float64) float64 { return v * math.Pi / 180 }
func dot(a, b vector) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func cross(a, b vector) vector {
	return vector{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}
func norm(a vector) float64 { return math.Sqrt(dot(a, a)) }
func unit(a vector) vector {
	n := norm(a)
	return vector{a.x / n, a.y / n, a.z / n}
}
func angle(a, b vector) float64 { return math.Atan2(norm(cross(a, b)), dot(a, b)) }
func tangentAt(p, v vector) vector {
	return vector{v.x - dot(v, p)*p.x, v.y - dot(v, p)*p.y, v.z - dot(v, p)*p.z}
}
func pointOnArc(p, a, b vector) bool {
	ab := angle(a, b)
	if math.Abs(ab-math.Pi) < 1e-12 {
		return false
	}
	normal := cross(a, b)
	if norm(normal) < 1e-14 {
		return false
	}
	normal = unit(normal)
	crossTrack := math.Asin(math.Min(1, math.Abs(dot(p, normal))))
	if crossTrack > 1e-10 {
		return false
	}
	projection := vector{
		x: p.x - dot(p, normal)*normal.x,
		y: p.y - dot(p, normal)*normal.y,
		z: p.z - dot(p, normal)*normal.z,
	}
	if norm(projection) < 1e-14 {
		return false
	}
	return onMinorArc(unit(projection), a, b)
}
func onMinorArc(p, a, b vector) bool {
	ab := angle(a, b)
	if ab < 1e-12 || ab >= math.Pi-1e-12 {
		return false
	}
	return math.Abs(angle(a, p)+angle(p, b)-ab) <= 1e-10
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func planarSignedArea(ring []Position) float64 {
	var area float64
	for i := range ring {
		next := ring[(i+1)%len(ring)]
		area += ring[i][0]*next[1] - next[0]*ring[i][1]
	}
	return area / 2
}
func reverse(ring []Position) {
	for i, j := 0, len(ring)-1; i < j; i, j = i+1, j-1 {
		ring[i], ring[j] = ring[j], ring[i]
	}
}

func hasSelfIntersection(ring []Position) bool {
	n := len(ring)
	for i := 0; i < n; i++ {
		a, b := vectorFor(ring[i]), vectorFor(ring[(i+1)%n])
		if angle(a, b) < 1e-12 || angle(a, b) >= math.Pi-1e-12 {
			return true
		}
		for j := i + 1; j < n; j++ {
			if j == i || j == (i+1)%n || i == (j+1)%n {
				continue
			}
			c, d := vectorFor(ring[j]), vectorFor(ring[(j+1)%n])
			if arcsIntersect(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

func hasAdjacentBacktrack(ring []Position) bool {
	for i := range ring {
		previous := vectorFor(ring[(i+len(ring)-1)%len(ring)])
		current := vectorFor(ring[i])
		next := vectorFor(ring[(i+1)%len(ring)])
		toPrevious := vector{previous.x - current.x, previous.y - current.y, previous.z - current.z}
		toNext := vector{next.x - current.x, next.y - current.y, next.z - current.z}
		if norm(toPrevious) < 1e-12 || norm(toNext) < 1e-12 {
			return true
		}
		toPrevious, toNext = unit(toPrevious), unit(toNext)
		if norm(cross(toPrevious, toNext)) < 1e-12 && dot(toPrevious, toNext) > 0 {
			return true
		}
	}
	return false
}

func arcsIntersect(a, b, c, d vector) bool {
	n1, n2 := cross(a, b), cross(c, d)
	axis := cross(n1, n2)
	if norm(axis) < 1e-12 {
		return onMinorArc(a, c, d) || onMinorArc(b, c, d) || onMinorArc(c, a, b) || onMinorArc(d, a, b)
	}
	p := unit(axis)
	return (onMinorArc(p, a, b) && onMinorArc(p, c, d)) || (onMinorArc(vector{-p.x, -p.y, -p.z}, a, b) && onMinorArc(vector{-p.x, -p.y, -p.z}, c, d))
}

func ValidateGeometry(g Geometry) error {
	encoded, err := json.Marshal(g)
	if err != nil || len(encoded) > 64<<10 {
		return fmt.Errorf("%w: geometry exceeds 64 KiB", ErrInvalidGeometry)
	}
	_, err = ParseGeometry(encoded)
	return err
}

// ValidateCanonical checks persisted geometry shape and numeric bounds without
// repeating the quadratic self-intersection validation done on writes.
func ValidateCanonical(g Geometry) error {
	switch g.Type {
	case "Circle":
		if g.Center == nil || g.RadiusM == nil || g.Coordinates != nil {
			return ErrInvalidGeometry
		}
		if _, err := parsePosition([]float64{g.Center[0], g.Center[1]}); err != nil {
			return ErrInvalidGeometry
		}
		if !finite(*g.RadiusM) || *g.RadiusM <= 0 || *g.RadiusM > MaxCircleRadiusM {
			return ErrInvalidGeometry
		}
		return nil
	case "Polygon":
		if g.Center != nil || g.RadiusM != nil || len(g.Coordinates) != 1 {
			return ErrInvalidGeometry
		}
		ring := g.Coordinates[0]
		if len(ring) < 4 || len(ring)-1 > MaxPolygonVertices || ring[0] != ring[len(ring)-1] {
			return ErrInvalidGeometry
		}
		points := ring[:len(ring)-1]
		unique := make(map[Position]struct{}, len(points))
		minLon, maxLon := points[0][0], points[0][0]
		for i, point := range points {
			if _, err := parsePosition([]float64{point[0], point[1]}); err != nil {
				return ErrInvalidGeometry
			}
			if _, exists := unique[point]; exists {
				return ErrInvalidGeometry
			}
			unique[point] = struct{}{}
			minLon = math.Min(minLon, point[0])
			maxLon = math.Max(maxLon, point[0])
			if math.Abs(point[0]-points[(i+1)%len(points)][0]) > 180 {
				return ErrInvalidGeometry
			}
		}
		if maxLon-minLon >= 180 || math.Abs(planarSignedArea(points)) < 1e-12 || planarSignedArea(points) < 0 {
			return ErrInvalidGeometry
		}
		return nil
	default:
		return ErrInvalidGeometry
	}
}
