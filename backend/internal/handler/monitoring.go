package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"geoentity/backend/internal/monitoring"
)

var deviceLimits = struct {
	sync.Mutex
	buckets map[string]limitBucket
	ops     int
}{buckets: map[string]limitBucket{}}

type limitBucket struct {
	start time.Time
	count int
}

func takeLimit(key string, max int, now time.Time) bool {
	deviceLimits.Lock()
	defer deviceLimits.Unlock()
	b := deviceLimits.buckets[key]
	if now.Sub(b.start) >= time.Minute || b.start.IsZero() {
		b = limitBucket{start: now}
	}
	b.count++
	deviceLimits.buckets[key] = b
	deviceLimits.ops++
	if deviceLimits.ops%128 == 0 {
		for k, v := range deviceLimits.buckets {
			if now.Sub(v.start) > 2*time.Minute {
				delete(deviceLimits.buckets, k)
			}
		}
	}
	if len(deviceLimits.buckets) > 20000 {
		for k, v := range deviceLimits.buckets {
			if now.Sub(v.start) > 2*time.Minute {
				delete(deviceLimits.buckets, k)
			}
		}
		for len(deviceLimits.buckets) > 20000 {
			var oldest string
			var oldestAt time.Time
			for k, v := range deviceLimits.buckets {
				if oldest == "" || v.start.Before(oldestAt) {
					oldest, oldestAt = k, v.start
				}
			}
			delete(deviceLimits.buckets, oldest)
		}
	}
	return b.count <= max
}

func (a *API) ingestObservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if a.monitor == nil {
		writeError(w, 500, "INTERNAL_ERROR", "An internal server error occurred", "")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, 400, "INVALID_JSON", "Content-Type must be application/json", "")
		return
	}
	remote := a.clientIP(r)
	now := time.Now()
	if !takeLimit("global", 600, now) || !takeLimit("ip:"+remote, 300, now) {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "RATE_LIMITED", "Request rate limit exceeded", "")
		return
	}
	if len(r.Header.Values("Authorization")) != 1 {
		writeError(w, 401, "DEVICE_NOT_AUTHORIZED", "Device not authorized", "")
		return
	}
	credential, ok := bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, 401, "DEVICE_NOT_AUTHORIZED", "Device not authorized", "")
		return
	}
	device, entity, credentialID, err := a.monitor.Authenticate(r.Context(), credential)
	if err != nil {
		if errors.Is(err, monitoring.ErrInactive) {
			writeError(w, 409, "ENTITY_NOT_ACTIVE", "Entity is not active", "")
		} else {
			writeError(w, 401, "DEVICE_NOT_AUTHORIZED", "Device not authorized", "")
		}
		return
	}
	if !takeLimit("device:"+device, 60, now) {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "RATE_LIMITED", "Request rate limit exceeded", "")
		return
	}
	if len(r.Header.Values("Idempotency-Key")) != 1 {
		writeError(w, 400, "VALIDATION_ERROR", "A single Idempotency-Key is required", "Idempotency-Key")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	dec := json.NewDecoder(r.Body)
	var fields map[string]json.RawMessage
	if err := dec.Decode(&fields); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, 413, "REQUEST_TOO_LARGE", "Request body exceeds 16 KiB", "")
		} else {
			writeError(w, 400, "INVALID_JSON", "Request body must contain one valid JSON object", "")
		}
		return
	}
	if fields == nil {
		writeError(w, 400, "INVALID_JSON", "Request body must be a JSON object", "")
		return
	}
	for key := range fields {
		if key != "sensor_id" && key != "value" && key != "unit" && key != "observed_at" && key != "metadata" {
			writeError(w, 400, "INVALID_JSON", "Request contains an unknown field", key)
			return
		}
	}
	for _, key := range []string{"sensor_id", "value", "unit", "observed_at"} {
		if _, ok := fields[key]; !ok {
			writeError(w, 400, "VALIDATION_ERROR", "Required field is missing", key)
			return
		}
	}
	if strings.TrimSpace(string(fields["value"])) == "null" {
		writeError(w, 400, "VALIDATION_ERROR", "value must be a finite number", "value")
		return
	}
	if raw, ok := fields["metadata"]; ok {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			writeError(w, 400, "VALIDATION_ERROR", "metadata must be a JSON object", "metadata")
			return
		}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, 400, "INVALID_JSON", "Request body must contain exactly one JSON object", "")
		return
	}
	encoded, _ := json.Marshal(fields)
	strict := json.NewDecoder(strings.NewReader(string(encoded)))
	strict.DisallowUnknownFields()
	var input monitoring.Input
	if err := strict.Decode(&input); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Observation fields have invalid types", "")
		return
	}
	result, duplicate, err := a.monitor.Ingest(r.Context(), device, entity, credentialID, r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		switch {
		case errors.Is(err, monitoring.ErrInvalid):
			writeError(w, 400, "VALIDATION_ERROR", "Observation is invalid", "")
		case errors.Is(err, monitoring.ErrInactive):
			writeError(w, 409, "ENTITY_NOT_ACTIVE", "Entity is not active", "")
		case errors.Is(err, monitoring.ErrConflict):
			writeError(w, 409, "IDEMPOTENCY_CONFLICT", "Observation conflicts with existing sensor state", "")
		case errors.Is(err, monitoring.ErrUnitConflict):
			writeError(w, 409, "SENSOR_UNIT_CONFLICT", "Sensor unit is immutable; use a new sensor ID", "unit")
		default:
			a.internalError(w, err)
		}
		return
	}
	result.Duplicate = duplicate
	status := 201
	if duplicate {
		status = 200
	}
	writeJSON(w, status, map[string]any{"data": result})
}

func (a *API) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "unknown"
	}
	for _, raw := range a.trustedProxyCIDRs {
		_, cidr, e := net.ParseCIDR(raw)
		if e == nil && cidr.Contains(peer) {
			if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
				return ip.String()
			}
		}
	}
	return peer.String()
}
func bearer(header string) (string, bool) {
	if strings.Count(header, " ") != 1 {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" || strings.ContainsAny(parts[1], " \t\r\n") {
		return "", false
	}
	return parts[1], true
}

func (a *API) monitoringAdmin(w http.ResponseWriter, r *http.Request) {
	if a.monitor == nil {
		writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if r.URL.Path == "/api/admin/devices" {
		values, ok := parseQueryValues(w, r, map[string]bool{"limit": true, "cursor": true})
		if !ok {
			return
		}
		limit := 50
		if raw := values.Get("limit"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > 200 {
				writeError(w, 400, "INVALID_QUERY", "Invalid device query", "limit")
				return
			}
			limit = n
		}
		before := ""
		if raw := values.Get("cursor"); raw != "" {
			decoded, e := base64.RawURLEncoding.DecodeString(raw)
			if e != nil || len(decoded) > 128 {
				writeError(w, 400, "INVALID_QUERY", "Invalid cursor", "cursor")
				return
			}
			before = string(decoded)
		}
		items, err := a.monitor.ListDevices(r.Context(), limit+1, before)
		if err != nil {
			a.internalError(w, err)
			return
		}
		next := ""
		if len(items) > limit {
			next = base64.RawURLEncoding.EncodeToString([]byte(items[limit-1].ID))
			items = items[:limit]
		}
		writeJSON(w, 200, map[string]any{"data": items, "next_cursor": next})
		return
	}
	if r.URL.Path == "/api/monitoring/latest" {
		entityID := r.URL.Query().Get("entity_id")
		if entityID == "" {
			writeError(w, 400, "INVALID_QUERY", "entity_id is required", "entity_id")
			return
		}
		a.monitorRows(w, r, entityID, true)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "entities" && parts[3] == "sensors" && parts[4] == "observations" {
		a.monitorRows(w, r, parts[2], false)
		return
	}
	writeError(w, 404, "NOT_FOUND", "Endpoint not found", "")
}
func (a *API) monitorRows(w http.ResponseWriter, r *http.Request, entityID string, latest bool) {
	q := r.URL.Query()
	allowed := map[string]bool{"entity_id": true, "from": true, "to": true, "sensor_id": true, "limit": true, "cursor": true}
	for k, v := range q {
		if !allowed[k] || len(v) != 1 {
			writeError(w, 400, "INVALID_QUERY", "Invalid history filters", k)
			return
		}
	}
	limit := 50
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 200 {
			writeError(w, 400, "INVALID_QUERY", "Limit must be between 1 and 200", "limit")
			return
		}
		limit = n
	}
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	var cursorParts []string
	if raw := q.Get("cursor"); raw != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil {
			writeError(w, 400, "INVALID_QUERY", "Invalid cursor", "cursor")
			return
		}
		cursorParts = strings.Split(string(decoded), "\x00")
		if latest {
			if len(cursorParts) != 5 || cursorParts[0] != "latest" || cursorParts[1] != entityID || cursorParts[2] != q.Get("sensor_id") || cursorParts[3] == "" || cursorParts[4] == "" {
				writeError(w, 400, "INVALID_QUERY", "Cursor does not match filters", "cursor")
				return
			}
		} else if len(cursorParts) != 6 || cursorParts[0] != entityID || cursorParts[3] != q.Get("sensor_id") {
			writeError(w, 400, "INVALID_QUERY", "Cursor does not match filters", "cursor")
			return
		}
		if !latest && q.Get("from") == "" && q.Get("to") == "" {
			var e1, e2 error
			from, e1 = time.Parse(time.RFC3339Nano, cursorParts[1])
			to, e2 = time.Parse(time.RFC3339Nano, cursorParts[2])
			if e1 != nil || e2 != nil {
				writeError(w, 400, "INVALID_QUERY", "Invalid cursor", "cursor")
				return
			}
		}
	}
	if !latest && (q.Get("from") != "" || q.Get("to") != "") {
		if q.Get("from") == "" || q.Get("to") == "" {
			writeError(w, 400, "INVALID_QUERY", "from and to must be provided together", "")
			return
		}
		var err error
		from, err = time.Parse(time.RFC3339, q.Get("from"))
		if err != nil {
			writeError(w, 400, "INVALID_QUERY", "Invalid from timestamp", "from")
			return
		}
		to, err = time.Parse(time.RFC3339, q.Get("to"))
		if err != nil || to.Before(from) || to.Sub(from) > 31*24*time.Hour {
			writeError(w, 400, "INVALID_QUERY", "Invalid time window", "to")
			return
		}
	}
	db := a.monitor.DB
	query := `SELECT id,entity_id,device_id,sensor_id,value,unit,observed_at,ingested_at FROM sensor_observations o WHERE entity_id=? AND (?='' OR sensor_id=?) AND NOT EXISTS (SELECT 1 FROM sensor_observations n WHERE n.entity_id=o.entity_id AND n.sensor_id=o.sensor_id AND (n.observed_at>o.observed_at OR (n.observed_at=o.observed_at AND n.id>o.id))) ORDER BY observed_at DESC,id DESC LIMIT ?`
	args := []any{entityID, q.Get("sensor_id"), q.Get("sensor_id"), limit + 1}
	if latest {
		cursorTime, cursorID := "", ""
		if len(cursorParts) > 0 {
			cursorTime, cursorID = cursorParts[3], cursorParts[4]
		}
		query = `SELECT id,entity_id,device_id,sensor_id,value,unit,observed_at,ingested_at FROM sensor_observations o WHERE entity_id=? AND (?='' OR sensor_id=?) AND NOT EXISTS (SELECT 1 FROM sensor_observations n WHERE n.entity_id=o.entity_id AND n.sensor_id=o.sensor_id AND (n.observed_at>o.observed_at OR (n.observed_at=o.observed_at AND n.id>o.id))) AND (?='' OR observed_at<? OR (observed_at=? AND id<?)) ORDER BY observed_at DESC,id DESC LIMIT ?`
		args = []any{entityID, q.Get("sensor_id"), q.Get("sensor_id"), cursorTime, cursorTime, cursorTime, cursorID, limit + 1}
	} else {
		cursorTime, cursorID := "", ""
		if len(cursorParts) > 0 {
			if cursorParts[1] != from.UTC().Format(time.RFC3339Nano) || cursorParts[2] != to.UTC().Format(time.RFC3339Nano) || cursorParts[4] == "" || cursorParts[5] == "" {
				writeError(w, 400, "INVALID_QUERY", "Cursor does not match filters", "cursor")
				return
			}
			cursorTime, cursorID = cursorParts[4], cursorParts[5]
		}
		query, args = monitoring.HistoryQuery(entityID, q.Get("sensor_id"), from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano), cursorTime, cursorID, limit+1)
	}
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, eid, did, sid, unit, observed, ingested string
		var value float64
		if err = rows.Scan(&id, &eid, &did, &sid, &value, &unit, &observed, &ingested); err != nil {
			a.internalError(w, err)
			return
		}
		out = append(out, map[string]any{"id": id, "entity_id": eid, "device_id": did, "sensor_id": sid, "value": value, "unit": unit, "observed_at": observed, "ingested_at": ingested})
	}
	if err = rows.Err(); err != nil {
		a.internalError(w, err)
		return
	}
	next := ""
	if len(out) > limit {
		m := out[limit-1]
		if latest {
			next = base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{"latest", entityID, q.Get("sensor_id"), m["observed_at"].(string), m["id"].(string)}, "\x00")))
		} else {
			next = base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{entityID, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano), q.Get("sensor_id"), m["observed_at"].(string), m["id"].(string)}, "\x00")))
		}
		out = out[:limit]
	}
	writeJSON(w, 200, map[string]any{"data": out, "next_cursor": next})
}
