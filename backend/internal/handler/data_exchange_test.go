package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
)

func openFixtureDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func rawRequest(api http.Handler, method, path, contentType string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	return res
}

func csvFixture(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Write(csvHeader)
	for _, row := range rows {
		writer.Write(row)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestCSVImportPreviewCommitMetadataAndAtomicActivity(t *testing.T) {
	api, store, path := newTestAPI(t)
	db := openFixtureDB(t, path)
	definition := request(api, http.MethodPost, "/api/attribute-definitions", `{"entity_type":"vehicle","key":"serial","label":"Serial","value_type":"string","required":true,"max_length":20}`)
	if definition.Code != 201 {
		t.Fatalf("definition: %d %s", definition.Code, definition.Body.String())
	}
	historicalDefinition := request(api, http.MethodPost, "/api/attribute-definitions", `{"entity_type":"iot_device","key":"device_id","label":"Device ID","value_type":"string","required":false,"max_length":20}`)
	if historicalDefinition.Code != 201 {
		t.Fatalf("historical definition: %d %s", historicalDefinition.Code, historicalDefinition.Body.String())
	}
	id := "00000000-0000-4000-8000-000000000701"
	data := csvFixture(t, [][]string{{id, "=SUM(1,2)", "vehicle", "Depot, east\nsecond line", "active", "-6.2", "106.8", "2026-11-03T00:00:00Z", "2026-11-03T00:00:00Z", `{"serial":"S-7"}`, `{"iot_device.device_id":"H-1"}`, `["Fleet"]`, `["Operations"]`}})
	preview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv; charset=utf-8", data, nil)
	if preview.Code != 200 {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	var previewEnvelope struct {
		Data importPreview `json:"data"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	if previewEnvelope.Data.TotalRows != 1 || previewEnvelope.Data.ValidRows != 1 || previewEnvelope.Data.InvalidRows != 0 || previewEnvelope.Data.PreviewSHA256 == "" || len(previewEnvelope.Data.NewCatalogItems) != 2 {
		t.Fatalf("preview DTO: %+v", previewEnvelope.Data)
	}
	if entities, _ := store.List(context.Background()); len(entities) != 0 {
		t.Fatal("preview wrote an entity")
	}
	var events, tags, categories int
	if err := db.QueryRow(`SELECT count(*) FROM activity_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM tags`).Scan(&tags); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM categories`).Scan(&categories); err != nil {
		t.Fatal(err)
	}
	if events+tags+categories != 0 {
		t.Fatalf("preview side effects: events=%d tags=%d categories=%d", events, tags, categories)
	}
	commit := rawRequest(api, http.MethodPost, "/api/imports/csv/commit?id_mode=preserve", "text/csv", data, map[string]string{"X-Preview-SHA256": previewEnvelope.Data.PreviewSHA256})
	if commit.Code != 201 {
		t.Fatalf("commit: %d %s", commit.Code, commit.Body.String())
	}
	entityResponse := request(api, http.MethodGet, "/api/entities/"+id, "")
	if entityResponse.Code != 200 || !strings.Contains(entityResponse.Body.String(), "Depot, east\\nsecond line") {
		t.Fatalf("entity roundtrip: %d %s", entityResponse.Code, entityResponse.Body.String())
	}
	attrs := request(api, http.MethodGet, "/api/entities/"+id+"/attributes", "")
	if attrs.Code != 200 || !strings.Contains(attrs.Body.String(), `"serial":"S-7"`) {
		t.Fatalf("attribute roundtrip: %d %s", attrs.Code, attrs.Body.String())
	}
	entityTags := request(api, http.MethodGet, "/api/entities/"+id+"/tags", "")
	if entityTags.Code != 200 || !strings.Contains(entityTags.Body.String(), "Fleet") {
		t.Fatalf("tag roundtrip: %d %s", entityTags.Code, entityTags.Body.String())
	}
	entityCategories := request(api, http.MethodGet, "/api/entities/"+id+"/categories", "")
	if entityCategories.Code != 200 || !strings.Contains(entityCategories.Body.String(), "Operations") {
		t.Fatalf("category roundtrip: %d %s", entityCategories.Code, entityCategories.Body.String())
	}
	activity := request(api, http.MethodGet, "/api/entities/"+id+"/activity", "")
	if activity.Code != 200 || !strings.Contains(activity.Body.String(), `"source":"import"`) || !strings.Contains(activity.Body.String(), `"format":"csv"`) {
		t.Fatalf("import activity: %d %s", activity.Code, activity.Body.String())
	}
	var actorKind string
	var actorID sql.NullString
	if err := db.QueryRow(`SELECT count(*),max(actor_kind),max(actor_id) FROM activity_events WHERE entity_id=? AND event_type='entity.created'`, id).Scan(&events, &actorKind, &actorID); err != nil || events != 1 || actorKind != "system" || actorID.Valid {
		t.Fatalf("activity events=%d err=%v", events, err)
	}

	machine := rawRequest(api, http.MethodGet, "/api/exports/entities.csv", "", nil, nil)
	if machine.Code != 200 || !strings.HasPrefix(machine.Header().Get("Content-Type"), "text/csv") || !strings.Contains(machine.Header().Get("Content-Disposition"), "geoentity-entities-") {
		t.Fatalf("CSV export headers: %d %v", machine.Code, machine.Header())
	}
	parsed, _ := csv.NewReader(bytes.NewReader(machine.Body.Bytes())).ReadAll()
	if len(parsed) != 2 || parsed[1][1] != "=SUM(1,2)" || parsed[1][3] != "Depot, east\nsecond line" || parsed[1][9] != `{"serial":"S-7"}` || parsed[1][10] != `{"iot_device.device_id":"H-1"}` {
		t.Fatalf("CSV export record: %#v", parsed)
	}
	safe := rawRequest(api, http.MethodGet, "/api/exports/entities.csv?spreadsheet_safe=true", "", nil, nil)
	safeRows, _ := csv.NewReader(bytes.NewReader(safe.Body.Bytes())).ReadAll()
	if len(safeRows) != 2 || safeRows[1][1] != "'=SUM(1,2)" || safeRows[1][5] != "-6.2" {
		t.Fatalf("spreadsheet-safe record: %#v", safeRows)
	}
	geo := rawRequest(api, http.MethodGet, "/api/exports/entities.geojson", "", nil, nil)
	if geo.Code != 200 || geo.Header().Get("Content-Type") != "application/geo+json; charset=utf-8" {
		t.Fatalf("GeoJSON export: %d %v", geo.Code, geo.Header())
	}
	var featureCollection struct {
		Type     string `json:"type"`
		Features []struct {
			ID       string `json:"id"`
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(geo.Body.Bytes(), &featureCollection); err != nil {
		t.Fatal(err)
	}
	if featureCollection.Type != "FeatureCollection" || len(featureCollection.Features) != 1 || featureCollection.Features[0].Geometry.Coordinates[0] != 106.8 || featureCollection.Features[0].Geometry.Coordinates[1] != -6.2 {
		t.Fatalf("GeoJSON coordinates/order: %+v", featureCollection)
	}
}

func TestGeoJSONImportPreviewAndCommitWithPointOrderAndAltitude(t *testing.T) {
	api, store, _ := newTestAPI(t)
	id := "00000000-0000-4000-8000-000000000702"
	body := []byte(fmt.Sprintf(`{"type":"FeatureCollection","features":[{"type":"Feature","id":%q,"geometry":{"type":"Point","coordinates":[123.4,-7.8,50]},"properties":{"name":"Sensor","type":"iot_device","description":"","status":"active","attributes":{},"previous_type_values":{},"tags":["IoT"],"categories":["Monitoring"]}}]}`, id))
	preview := rawRequest(api, http.MethodPost, "/api/imports/geojson/preview", "application/geo+json", body, nil)
	if preview.Code != 200 {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	var env struct {
		Data importPreview `json:"data"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.ValidRows != 1 || len(env.Data.Warnings) < 1 || env.Data.Warnings[0].Code != "ALTITUDE_DISCARDED" {
		t.Fatalf("preview result: %+v", env.Data)
	}
	if rows, _ := store.List(context.Background()); len(rows) != 0 {
		t.Fatal("GeoJSON preview wrote data")
	}
	commit := rawRequest(api, http.MethodPost, "/api/imports/geojson/commit", "application/geo+json", body, map[string]string{"X-Preview-SHA256": env.Data.PreviewSHA256})
	if commit.Code != 201 {
		t.Fatalf("commit: %d %s", commit.Code, commit.Body.String())
	}
	get := request(api, http.MethodGet, "/api/entities/"+id, "")
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"latitude":-7.8`) || !strings.Contains(get.Body.String(), `"longitude":123.4`) {
		t.Fatalf("GeoJSON coordinate mapping: %d %s", get.Code, get.Body.String())
	}
	geo := rawRequest(api, http.MethodGet, "/api/exports/entities.geojson", "", nil, nil)
	if !strings.Contains(geo.Body.String(), `[123.4,-7.8]`) {
		t.Fatalf("GeoJSON export order: %s", geo.Body.String())
	}
}

func TestImportPreviewValidationNoWritesAndCommitFingerprintConflicts(t *testing.T) {
	api, store, _ := newTestAPI(t)
	id := "00000000-0000-4000-8000-000000000703"
	data := csvFixture(t, [][]string{{id, "Broken", "sensor", "", "unknown", "999", "200", "", "", "{}", "{}", "[]", "[]"}})
	preview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", data, nil)
	if preview.Code != 200 {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	var env struct {
		Data importPreview `json:"data"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &env)
	if env.Data.InvalidRows != 1 || len(env.Data.Errors) < 3 {
		t.Fatalf("invalid preview details: %+v", env.Data)
	}
	if rows, _ := store.List(context.Background()); len(rows) != 0 {
		t.Fatal("invalid preview wrote entities")
	}
	duplicateRows := csvFixture(t, [][]string{
		{id, "First duplicate", "vehicle", "", "active", "1", "2", "", "", "{}", "{}", "[]", "[]"},
		{id, "Second duplicate", "vehicle", "", "active", "3", "4", "", "", "{}", "{}", "[]", "[]"},
	})
	duplicatePreview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", duplicateRows, nil)
	if duplicatePreview.Code != 200 || !strings.Contains(duplicatePreview.Body.String(), "DUPLICATE_ID") {
		t.Fatalf("duplicate IDs: %d %s", duplicatePreview.Code, duplicatePreview.Body.String())
	}
	good := csvFixture(t, [][]string{{id, "Conflict", "vehicle", "", "active", "1", "2", "", "", "{}", "{}", "[]", "[]"}})
	goodPreview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", good, nil)
	var goodEnv struct {
		Data importPreview `json:"data"`
	}
	_ = json.Unmarshal(goodPreview.Body.Bytes(), &goodEnv)
	changed := bytes.Replace(good, []byte("Conflict"), []byte("Changed"), 1)
	stale := rawRequest(api, http.MethodPost, "/api/imports/csv/commit", "text/csv", changed, map[string]string{"X-Preview-SHA256": goodEnv.Data.PreviewSHA256})
	if stale.Code != 409 || !strings.Contains(stale.Body.String(), "PREVIEW_STALE") {
		t.Fatalf("fingerprint mismatch: %d %s", stale.Code, stale.Body.String())
	}
	item := entity.Entity{ID: id, Name: "Earlier writer", Type: "vehicle", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := store.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	conflict := rawRequest(api, http.MethodPost, "/api/imports/csv/commit", "text/csv", good, map[string]string{"X-Preview-SHA256": goodEnv.Data.PreviewSHA256})
	if conflict.Code != 409 {
		t.Fatalf("ID conflict: %d %s", conflict.Code, conflict.Body.String())
	}
}

func TestImportActivityFailureRollsBackBatchAndLimitsAreEnforced(t *testing.T) {
	api, store, path := newTestAPI(t)
	db := openFixtureDB(t, path)
	rows := [][]string{{"00000000-0000-4000-8000-000000000704", "Atomic one", "facility", "", "active", "1", "2", "", "", "{}", "{}", `["Rollback tag"]`, `["Rollback category"]`}, {"00000000-0000-4000-8000-000000000705", "Atomic two", "facility", "", "active", "3", "4", "", "", "{}", "{}", `["Rollback tag"]`, `["Rollback category"]`}}
	data := csvFixture(t, rows)
	preview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", data, nil)
	var env struct {
		Data importPreview `json:"data"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &env)
	if _, err := db.Exec(`CREATE TRIGGER reject_import_activity BEFORE INSERT ON activity_events WHEN NEW.event_type='entity.created' BEGIN SELECT RAISE(ABORT,'injected import activity failure'); END`); err != nil {
		t.Fatal(err)
	}
	commit := rawRequest(api, http.MethodPost, "/api/imports/csv/commit", "text/csv", data, map[string]string{"X-Preview-SHA256": env.Data.PreviewSHA256})
	if commit.Code != 500 {
		t.Fatalf("injected failure: %d %s", commit.Code, commit.Body.String())
	}
	if items, _ := store.List(context.Background()); len(items) != 0 {
		t.Fatalf("partial entities after rollback: %d", len(items))
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM activity_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial events=%d err=%v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM tags`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial tags=%d err=%v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM categories`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial categories=%d err=%v", count, err)
	}
	oversized := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", bytes.Repeat([]byte("x"), maxImportBytes+1), nil)
	if oversized.Code != 413 {
		t.Fatalf("oversize: %d %s", oversized.Code, oversized.Body.String())
	}
	badMedia := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "application/json", []byte("{}"), nil)
	if badMedia.Code != 415 {
		t.Fatalf("media type: %d %s", badMedia.Code, badMedia.Body.String())
	}
	badJSON := rawRequest(api, http.MethodPost, "/api/imports/geojson/preview", "application/geo+json", []byte(`{"type":"FeatureCollection","type":"FeatureCollection","features":[]}`), nil)
	if badJSON.Code != 400 {
		t.Fatalf("duplicate JSON key: %d %s", badJSON.Code, badJSON.Body.String())
	}
}

func TestCommitRechecksConcurrentIDConflictAndRollsBackBatch(t *testing.T) {
	api, store, path := newTestAPI(t)
	db := openFixtureDB(t, path)
	firstID := "00000000-0000-4000-8000-000000000708"
	secondID := "00000000-0000-4000-8000-000000000709"
	data := csvFixture(t, [][]string{
		{firstID, "First", "facility", "", "active", "1", "2", "", "", "{}", "{}", `["Batch tag"]`, `["Batch category"]`},
		{secondID, "Second", "facility", "", "active", "3", "4", "", "", "{}", "{}", `["Batch tag"]`, `["Batch category"]`},
	})
	preview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", data, nil)
	var env struct {
		Data importPreview `json:"data"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &env); err != nil || env.Data.InvalidRows != 0 {
		t.Fatalf("preview: %d %+v err=%v", preview.Code, env.Data, err)
	}
	trigger := `CREATE TRIGGER concurrent_import_conflict BEFORE INSERT ON entities WHEN NEW.id='` + firstID + `' BEGIN INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES('` + secondID + `','Concurrent writer','facility','','active',3,4,'2026-11-03T00:00:00Z','2026-11-03T00:00:00Z'); END`
	if _, err := db.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	commit := rawRequest(api, http.MethodPost, "/api/imports/csv/commit", "text/csv", data, map[string]string{"X-Preview-SHA256": env.Data.PreviewSHA256})
	if commit.Code != 409 || !strings.Contains(commit.Body.String(), "IMPORT_CONFLICT") {
		t.Fatalf("concurrent conflict: %d %s", commit.Code, commit.Body.String())
	}
	if items, _ := store.List(context.Background()); len(items) != 0 {
		t.Fatalf("entities remained after rollback: %d", len(items))
	}
	var tags, categories int
	if err := db.QueryRow(`SELECT count(*) FROM tags`).Scan(&tags); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM categories`).Scan(&categories); err != nil {
		t.Fatal(err)
	}
	if tags != 0 || categories != 0 {
		t.Fatalf("catalog rows remained after rollback: tags=%d categories=%d", tags, categories)
	}
}

func TestExportsAreCompleteBeyondListPageAndApplyFilters(t *testing.T) {
	api, _, path := newTestAPI(t)
	db := openFixtureDB(t, path)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO entities(id,name,type,description,status,latitude,longitude,created_at,updated_at) VALUES(?,?,?,'','active',0,0,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 125; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", 800+i)
		if _, err := stmt.Exec(id, fmt.Sprintf("Asset %03d", i), "facility", "2026-11-03T00:00:00Z", "2026-11-03T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	page := request(api, http.MethodGet, "/api/entities/query?limit=100", "")
	if !strings.Contains(page.Body.String(), `"matching_count":125`) {
		t.Fatalf("query page count: %s", page.Body.String())
	}
	csvResponse := rawRequest(api, http.MethodGet, "/api/exports/entities.csv?type=facility", "", nil, nil)
	records, err := csv.NewReader(bytes.NewReader(csvResponse.Body.Bytes())).ReadAll()
	if err != nil || len(records) != 126 {
		t.Fatalf("CSV complete export records=%d err=%v", len(records), err)
	}
	geoResponse := rawRequest(api, http.MethodGet, "/api/exports/entities.geojson?status=active", "", nil, nil)
	var features struct {
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(geoResponse.Body.Bytes(), &features); err != nil || len(features.Features) != 125 {
		t.Fatalf("GeoJSON complete export count=%d err=%v", len(features.Features), err)
	}
	limited := rawRequest(api, http.MethodGet, "/api/exports/entities.csv?spreadsheet_safe=maybe", "", nil, nil)
	if limited.Code != 400 {
		t.Fatalf("invalid export query: %d %s", limited.Code, limited.Body.String())
	}
}

func TestEmptyCSVAndGeoJSONCoordinateValidation(t *testing.T) {
	api, _, _ := newTestAPI(t)
	empty := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", []byte("name,type,status,latitude,longitude\r\n"), nil)
	if empty.Code != 400 {
		t.Fatalf("empty CSV: %d %s", empty.Code, empty.Body.String())
	}
	for _, coordinate := range []string{`[200,10]`, `[10,95]`, `null`} {
		body := []byte(`{"type":"Feature","geometry":{"type":"Point","coordinates":` + coordinate + `},"properties":{"name":"Bad","type":"vehicle","status":"active"}}`)
		response := rawRequest(api, http.MethodPost, "/api/imports/geojson/preview", "application/geo+json", body, nil)
		if response.Code != 200 || !strings.Contains(response.Body.String(), `"invalid_rows":1`) {
			t.Errorf("coords %s => %d %s", coordinate, response.Code, response.Body.String())
		}
	}
	numericID := []byte(`{"type":"Feature","id":42,"geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"Bad ID","type":"vehicle","status":"active"}}`)
	response := rawRequest(api, http.MethodPost, "/api/imports/geojson/preview", "application/geo+json", numericID, nil)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "INVALID_ID") {
		t.Fatalf("numeric GeoJSON ID: %d %s", response.Code, response.Body.String())
	}
}

func TestImportBodyMetadataLimitsAreBounded(t *testing.T) {
	api, _, _ := newTestAPI(t)
	large := strings.Repeat("x", maxImportField+1)
	data := csvFixture(t, [][]string{{"00000000-0000-4000-8000-000000000706", "Long", "facility", large, "active", "1", "2", "", "", "{}", "{}", "[]", "[]"}})
	if len(data) > maxImportBytes {
		t.Skip("overall request cap covers the individual field limit")
	}
	response := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", data, nil)
	if response.Code != 413 || !strings.Contains(response.Body.String(), "IMPORT_LIMIT_EXCEEDED") {
		t.Fatalf("field limit: %d %s", response.Code, response.Body.String())
	}
}

func TestImportRowLimitsMalformedCSVAndRegeneratedIDPreview(t *testing.T) {
	api, _, _ := newTestAPI(t)
	rows := make([][]string, maxImportRows+1)
	for i := range rows {
		rows[i] = []string{"", fmt.Sprintf("Asset %d", i), "facility", "", "active", "1", "2", "", "", "{}", "{}", "[]", "[]"}
	}
	tooMany := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", csvFixture(t, rows), nil)
	if tooMany.Code != 413 || !strings.Contains(tooMany.Body.String(), "IMPORT_LIMIT_EXCEEDED") {
		t.Fatalf("row limit: %d %s", tooMany.Code, tooMany.Body.String())
	}
	malformed := rawRequest(api, http.MethodPost, "/api/imports/csv/preview", "text/csv", []byte("name,type,status,latitude,longitude\n\"unterminated,facility,active,1,2"), nil)
	if malformed.Code != 400 {
		t.Fatalf("malformed CSV: %d %s", malformed.Code, malformed.Body.String())
	}
	id := "00000000-0000-4000-8000-000000000707"
	body := csvFixture(t, [][]string{{id, "Regenerated", "facility", "", "active", "1", "2", "", "", "{}", "{}", "[]", "[]"}})
	preview := rawRequest(api, http.MethodPost, "/api/imports/csv/preview?id_mode=regenerate", "text/csv", body, nil)
	var env struct {
		Data importPreview `json:"data"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if preview.Code != 200 || env.Data.InvalidRows != 0 || len(env.Data.IDMap) != 1 || env.Data.IDMap[0]["source_id"] != id || env.Data.IDMap[0]["entity_id"] == id {
		t.Fatalf("regenerated ID preview: %d %+v", preview.Code, env.Data)
	}
	commit := rawRequest(api, http.MethodPost, "/api/imports/csv/commit?id_mode=regenerate", "text/csv", body, map[string]string{"X-Preview-SHA256": env.Data.PreviewSHA256})
	if commit.Code != 201 || !strings.Contains(commit.Body.String(), `"source_id":"`+id+`"`) {
		t.Fatalf("regenerated ID commit: %d %s", commit.Code, commit.Body.String())
	}
}
