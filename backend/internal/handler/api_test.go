package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"
	_ "modernc.org/sqlite"
)

func newTestAPI(t *testing.T) (*API, *repository.SQLite, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "entities.db")
	store, err := repository.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close repository: %v", err)
		}
	})
	return New(store, slog.New(slog.NewTextHandler(io.Discard, nil))), store, dbPath
}

func request(api http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, req)
	return response
}

func TestHealthAndEmptyList(t *testing.T) {
	api, _, _ := newTestAPI(t)
	response := request(api, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %d %s", response.Code, response.Body.String())
	}

	response = request(api, http.MethodGet, "/api/entities", "")
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"data":[]}` {
		t.Fatalf("unexpected empty list: %d %s", response.Code, response.Body.String())
	}
}

func TestServerQueryAndMapEndpointsRemainSeparateFromV1List(t *testing.T) {
	api, store, _ := newTestAPI(t)
	createdAt := time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)
	items := []entity.Entity{
		{ID: "00000000-0000-4000-8000-000000000001", Name: "Truck Alpha", Type: "vehicle", Description: "north yard", Status: "active", Latitude: -6.2, Longitude: 106.8, CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "00000000-0000-4000-8000-000000000002", Name: "Truck Beta", Type: "vehicle", Description: "south yard", Status: "inactive", Latitude: -6.3, Longitude: 106.9, CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "00000000-0000-4000-8000-000000000003", Name: "Depot", Type: "facility", Description: "warehouse", Status: "active", Latitude: -6.4, Longitude: 106.7, CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for _, item := range items {
		if err := store.Create(context.Background(), item); err != nil {
			t.Fatal(err)
		}
	}

	legacy := request(api, http.MethodGet, "/api/entities", "")
	if legacy.Code != http.StatusOK || strings.Contains(legacy.Body.String(), `"meta"`) {
		t.Fatalf("V1 list changed: %d %s", legacy.Code, legacy.Body.String())
	}
	var legacyEnvelope struct {
		Data []entity.Entity `json:"data"`
	}
	if err := json.Unmarshal(legacy.Body.Bytes(), &legacyEnvelope); err != nil || len(legacyEnvelope.Data) != 3 {
		t.Fatalf("V1 list data=%+v err=%v", legacyEnvelope, err)
	}

	first := request(api, http.MethodGet, "/api/entities/query?limit=1", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first query page: %d %s", first.Code, first.Body.String())
	}
	var page struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			Limit         int            `json:"limit"`
			NextCursor    string         `json:"next_cursor"`
			MatchingCount int            `json:"matching_count"`
			StatusCounts  map[string]int `json:"status_counts"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Meta.Limit != 1 || page.Meta.NextCursor == "" || page.Meta.MatchingCount != 3 {
		t.Fatalf("unexpected first page: %+v", page)
	}
	second := request(api, http.MethodGet, "/api/entities/query?limit=1&cursor="+url.QueryEscape(page.Meta.NextCursor), "")
	if second.Code != http.StatusOK || strings.Contains(second.Body.String(), page.Data[0].ID) {
		t.Fatalf("second page: %d %s", second.Code, second.Body.String())
	}

	mapResponse := request(api, http.MethodGet, "/api/entities/map?west=100&south=-10&east=110&north=0", "")
	if mapResponse.Code != http.StatusOK {
		t.Fatalf("map response: %d %s", mapResponse.Code, mapResponse.Body.String())
	}
	var mapEnvelope struct {
		Data []repository.MapEntity `json:"data"`
	}
	if err := json.Unmarshal(mapResponse.Body.Bytes(), &mapEnvelope); err != nil || len(mapEnvelope.Data) != 3 {
		t.Fatalf("map dataset=%+v err=%v", mapEnvelope, err)
	}

	filteredMap := request(api, http.MethodGet, "/api/entities/map?west=100&south=-10&east=110&north=0&q=truck&type=vehicle&status=inactive", "")
	if filteredMap.Code != http.StatusOK || !strings.Contains(filteredMap.Body.String(), items[1].ID) || strings.Contains(filteredMap.Body.String(), items[0].ID) {
		t.Fatalf("map filters differ: %d %s", filteredMap.Code, filteredMap.Body.String())
	}
	extent := request(api, http.MethodGet, "/api/entities/map/extent?q=truck", "")
	if extent.Code != http.StatusOK || !strings.Contains(extent.Body.String(), `"count":2`) {
		t.Fatalf("map extent: %d %s", extent.Code, extent.Body.String())
	}

	// While viewing page 2, map endpoint still returns all 3 markers
	mapWhileOnPage2 := request(api, http.MethodGet, "/api/entities/map?west=100&south=-10&east=110&north=0", "")
	if mapWhileOnPage2.Code != http.StatusOK || !strings.Contains(mapWhileOnPage2.Body.String(), items[0].ID) || !strings.Contains(mapWhileOnPage2.Body.String(), items[1].ID) || !strings.Contains(mapWhileOnPage2.Body.String(), items[2].ID) {
		t.Fatalf("map markers were improperly tied to list page: %s", mapWhileOnPage2.Body.String())
	}

	// Deletion removes from map and list
	del := request(api, http.MethodDelete, "/api/entities/"+items[1].ID, "")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", del.Code)
	}
	mapAfterDel := request(api, http.MethodGet, "/api/entities/map?west=100&south=-10&east=110&north=0", "")
	if strings.Contains(mapAfterDel.Body.String(), items[1].ID) {
		t.Fatalf("deleted entity still returned in map dataset: %s", mapAfterDel.Body.String())
	}
}

func TestQueryEndpointRejectsInvalidAndRepeatedParameters(t *testing.T) {
	api, _, _ := newTestAPI(t)
	for _, path := range []string{
		"/api/entities/query?limit=0",
		"/api/entities/query?limit=",
		"/api/entities/query?limit=101",
		"/api/entities/query?type=sensor",
		"/api/entities/query?cursor=not-a-cursor",
		"/api/entities/query?q=a&q=b",
		"/api/entities/query?unknown=value",
	} {
		response := request(api, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("%s => %d %s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/entities/map?west=100&south=10&east=110&north=0",
		"/api/entities/map?west=-181&south=-10&east=110&north=0",
		"/api/entities/map?west=100&south=-10&east=110",
	} {
		response := request(api, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `INVALID_BOUNDS`) {
			t.Errorf("%s => %d %s", path, response.Code, response.Body.String())
		}
	}
	response := request(api, http.MethodPost, "/api/entities/query", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestEntityCRUDAndPersistence(t *testing.T) {
	api, store, dbPath := newTestAPI(t)
	body := `{"name":"  Depot  ","type":"facility","description":"West gate","status":"active","latitude":-90,"longitude":180}`
	createdResponse := request(api, http.MethodPost, "/api/entities", body)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var createdEnvelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &createdEnvelope); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	created := createdEnvelope.Data
	if created.ID == "" || created.Name != "Depot" {
		t.Fatalf("created entity was not normalized/generated: %+v", created)
	}
	if created.CreatedAt.Location() != time.UTC || created.UpdatedAt.Location() != time.UTC {
		t.Fatalf("timestamps are not UTC: %+v", created)
	}

	gotResponse := request(api, http.MethodGet, "/api/entities/"+created.ID, "")
	if gotResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", gotResponse.Code, gotResponse.Body.String())
	}

	updatedResponse := request(api, http.MethodPut, "/api/entities/"+created.ID, `{"name":"Depot 2","type":"facility","description":"Updated","status":"maintenance","latitude":0,"longitude":0}`)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	var updatedEnvelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal(updatedResponse.Body.Bytes(), &updatedEnvelope); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updatedEnvelope.Data.Name != "Depot 2" || updatedEnvelope.Data.Latitude != 0 || updatedEnvelope.Data.Status != "maintenance" {
		t.Fatalf("update not reflected: %+v", updatedEnvelope.Data)
	}
	if !updatedEnvelope.Data.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("updated_at did not advance: before=%s after=%s", created.UpdatedAt, updatedEnvelope.Data.UpdatedAt)
	}

	listResponse := request(api, http.MethodGet, "/api/entities", "")
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"name":"Depot 2"`) {
		t.Fatalf("list did not include updated item: %d %s", listResponse.Code, listResponse.Body.String())
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close store before reopen: %v", err)
	}
	reopened, err := repository.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen repository: %v", err)
	}
	persisted, err := reopened.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get persisted entity: %v", err)
	}
	if persisted.Name != "Depot 2" || persisted.Description != "Updated" {
		t.Fatalf("updated data was not persisted: %+v", persisted)
	}
	deleteAPI := New(reopened, slog.New(slog.NewTextHandler(io.Discard, nil)))
	deleteResponse := request(deleteAPI, http.MethodDelete, "/api/entities/"+created.ID, "")
	if deleteResponse.Code != http.StatusNoContent || deleteResponse.Body.Len() != 0 {
		t.Fatalf("delete response = %d %q", deleteResponse.Code, deleteResponse.Body.String())
	}
	missingResponse := request(deleteAPI, http.MethodGet, "/api/entities/"+created.ID, "")
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("deleted entity get status = %d", missingResponse.Code)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened repository: %v", err)
	}
}

func TestV1APICompatibilityAfterLegacyDatabaseBaseline(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE entities (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL CHECK(length(trim(name)) > 0),
		type TEXT NOT NULL CHECK(type IN ('vehicle', 'iot_device', 'facility', 'other')),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK(status IN ('active', 'inactive', 'maintenance')),
		latitude REAL NOT NULL CHECK(latitude >= -90 AND latitude <= 90),
		longitude REAL NOT NULL CHECK(longitude >= -180 AND longitude <= 180),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX idx_entities_created_at ON entities(created_at, id);
	INSERT INTO entities VALUES ('00000000-0000-4000-8000-000000000099','Legacy facility','facility','Original data','active',12.5,-45.25,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := repository.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open and baseline V1 database: %v", err)
	}
	defer store.Close()
	api := New(store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	list := request(api, http.MethodGet, "/api/entities", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"data":[`) || !strings.Contains(list.Body.String(), `"id":"00000000-0000-4000-8000-000000000099"`) || !strings.Contains(list.Body.String(), `"description":"Original data"`) {
		t.Fatalf("legacy list contract changed: %d %s", list.Code, list.Body.String())
	}
	get := request(api, http.MethodGet, "/api/entities/00000000-0000-4000-8000-000000000099", "")
	if get.Code != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(get.Body.String()), `{"data":`) {
		t.Fatalf("legacy detail contract changed: %d %s", get.Code, get.Body.String())
	}
	updated := request(api, http.MethodPut, "/api/entities/00000000-0000-4000-8000-000000000099", `{"name":"Legacy facility","type":"facility","description":"Updated through V1 API","status":"maintenance","latitude":12.5,"longitude":-45.25}`)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"description":"Updated through V1 API"`) {
		t.Fatalf("legacy update contract changed: %d %s", updated.Code, updated.Body.String())
	}
	deleted := request(api, http.MethodDelete, "/api/entities/00000000-0000-4000-8000-000000000099", "")
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("legacy delete contract changed: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestCreateValidationRejectsInvalidInputWithoutWriting(t *testing.T) {
	api, store, _ := newTestAPI(t)
	tests := []struct{ name, body, field string }{
		{"empty name", `{"name":"  ","type":"vehicle","status":"active","latitude":0,"longitude":0}`, "name"},
		{"unknown type", `{"name":"x","type":"plane","status":"active","latitude":0,"longitude":0}`, "type"},
		{"unknown status", `{"name":"x","type":"vehicle","status":"broken","latitude":0,"longitude":0}`, "status"},
		{"latitude low", `{"name":"x","type":"vehicle","status":"active","latitude":-90.01,"longitude":0}`, "latitude"},
		{"latitude high", `{"name":"x","type":"vehicle","status":"active","latitude":90.01,"longitude":0}`, "latitude"},
		{"longitude low", `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":-180.01}`, "longitude"},
		{"longitude high", `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":180.01}`, "longitude"},
		{"missing latitude", `{"name":"x","type":"vehicle","status":"active","longitude":0}`, "latitude"},
		{"missing longitude", `{"name":"x","type":"vehicle","status":"active","latitude":0}`, "longitude"},
		{"unknown field", `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":0,"id":"spoof"}`, ""},
		{"malformed json", `{`, ""},
		{"multiple json values", `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":0} {}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := request(api, http.MethodPost, "/api/entities", tt.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var envelope struct {
				Error struct {
					Code  string `json:"code"`
					Field string `json:"field"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if envelope.Error.Code == "" {
				t.Fatal("missing error code")
			}
			if tt.field != "" && envelope.Error.Field != tt.field {
				t.Fatalf("field = %q, want %q", envelope.Error.Field, tt.field)
			}
		})
	}
	items, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list after invalid requests: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("invalid requests wrote %d rows", len(items))
	}
}

func TestIDsNotFoundMethodsAndErrorRedaction(t *testing.T) {
	api, _, _ := newTestAPI(t)
	for _, path := range []string{"/api/entities/nope", "/api/entities/"} {
		response := request(api, http.MethodGet, path, "")
		if path == "/api/entities/" {
			if response.Code != http.StatusNotFound {
				t.Fatalf("path %q status = %d", path, response.Code)
			}
		} else if response.Code != http.StatusBadRequest {
			t.Fatalf("path %q status = %d", path, response.Code)
		}
	}
	validMissing := "/api/entities/00000000-0000-4000-8000-000000000001"
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		body := ""
		if method == http.MethodPut {
			body = `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":0}`
		}
		response := request(api, method, validMissing, body)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s missing ID status = %d, body = %s", method, response.Code, response.Body.String())
		}
	}
	response := request(api, http.MethodPatch, "/api/entities", "")
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") == "" {
		t.Fatalf("method response = %d, headers=%v", response.Code, response.Header())
	}
	response = request(api, http.MethodPost, "/api/entities", `{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":0}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("setup create failed: %d", response.Code)
	}
}

type failingStore struct{ err error }

func (f failingStore) List(context.Context) ([]entity.Entity, error) { return nil, f.err }
func (f failingStore) Get(context.Context, string) (entity.Entity, error) {
	return entity.Entity{}, f.err
}
func (f failingStore) Create(context.Context, entity.Entity) error { return f.err }
func (f failingStore) Update(context.Context, entity.Entity) error { return f.err }
func (f failingStore) Delete(context.Context, string) error        { return f.err }

func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	api := New(failingStore{err: errors.New("secret database path /private/store.db")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := request(api, http.MethodGet, "/api/entities", "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "/private") {
		t.Fatalf("internal detail leaked: %s", response.Body.String())
	}
}

func TestRequestRejectsWrongContentTypeAndNonFiniteNumbers(t *testing.T) {
	api, _, _ := newTestAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/entities", bytes.NewBufferString(`{"name":"x","type":"vehicle","status":"active","latitude":0,"longitude":0}`))
	response := httptest.NewRecorder()
	api.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing content type status = %d", response.Code)
	}
	for _, body := range []string{
		`{"name":"x","type":"vehicle","status":"active","latitude":NaN,"longitude":0}`,
		`{"name":"x","type":"vehicle","status":"active","latitude":Infinity,"longitude":0}`,
	} {
		response := request(api, http.MethodPost, "/api/entities", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("non-finite JSON status = %d, body = %s", response.Code, response.Body.String())
		}
	}
}

func TestAdvancedMetadataAndActivityLifecycle(t *testing.T) {
	api, _, _ := newTestAPI(t)
	created := request(api, http.MethodPost, "/api/entities", `{"name":"Asset","type":"vehicle","description":"","status":"active","latitude":1,"longitude":2}`)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var env struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	id := env.Data.ID
	df := request(api, http.MethodPost, "/api/attribute-definitions", `{"entity_type":"vehicle","key":"year","label":"Year","value_type":"number","required":true,"min_number":1900,"max_number":2100}`)
	if df.Code != 201 {
		t.Fatalf("definition: %d %s", df.Code, df.Body.String())
	}
	bad := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", `{"values":{}}`)
	if bad.Code != 400 {
		t.Fatalf("required validation status %d", bad.Code)
	}
	attrs := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", `{"values":{"year":2024}}`)
	if attrs.Code != 200 {
		t.Fatalf("attributes: %d %s", attrs.Code, attrs.Body.String())
	}
	// A legacy V1 update that omits metadata still succeeds, logs an event, and preserves typed values.
	v1update := request(api, http.MethodPut, "/api/entities/"+id, `{"name":"Asset moved","type":"facility","description":"","status":"active","latitude":3,"longitude":4}`)
	if v1update.Code != 200 {
		t.Fatalf("legacy update: %d %s", v1update.Code, v1update.Body.String())
	}
	previous := request(api, http.MethodGet, "/api/entities/"+id+"/attributes", "")
	if previous.Code != 200 || !strings.Contains(previous.Body.String(), `"vehicle.year":2024`) {
		t.Fatalf("type change did not preserve prior values: %d %s", previous.Code, previous.Body.String())
	}
	tag := request(api, http.MethodPost, "/api/tags", `{"name":"  Fleet   Unit "}`)
	if tag.Code != 201 {
		t.Fatalf("tag: %d %s", tag.Code, tag.Body.String())
	}
	var catalog struct {
		Data repository.CatalogItem `json:"data"`
	}
	_ = json.Unmarshal(tag.Body.Bytes(), &catalog)
	assign := request(api, http.MethodPut, "/api/entities/"+id+"/tags", `{"tag_ids":["`+catalog.Data.ID+`","`+catalog.Data.ID+`"]}`)
	if assign.Code != 200 {
		t.Fatalf("assign: %d %s", assign.Code, assign.Body.String())
	}
	assignAgain := request(api, http.MethodPut, "/api/entities/"+id+"/tags", `{"tag_ids":["`+catalog.Data.ID+`"]}`)
	if assignAgain.Code != 200 {
		t.Fatalf("repeat assign: %d", assignAgain.Code)
	}
	page := request(api, http.MethodGet, "/api/entities/"+id+"/activity?limit=100", "")
	if page.Code != 200 {
		t.Fatalf("activity: %d %s", page.Code, page.Body.String())
	}
	var events struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if len(events.Data) != 4 {
		t.Fatalf("expected create, update, attribute, tag events; got %d: %s", len(events.Data), page.Body.String())
	}
	// Exercise the delete repository path directly here to retain its diagnostic.
	if err := api.store.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete repository: %v", err)
	}
	history := request(api, http.MethodGet, "/api/entities/"+id+"/activity", "")
	if history.Code != 200 || !strings.Contains(history.Body.String(), "entity.deleted") {
		t.Fatalf("deleted history unavailable: %d %s", history.Code, history.Body.String())
	}
}

func TestContractEntityTypesAreConsistentForEntitiesAndDefinitions(t *testing.T) {
	api, store, _ := newTestAPI(t)
	allowed := []string{"vehicle", "iot_device", "facility", "other"}
	for index, typ := range allowed {
		body := `{"name":"type-` + typ + `","type":"` + typ + `","status":"active","latitude":0,"longitude":0}`
		if response := request(api, http.MethodPost, "/api/entities", body); response.Code != http.StatusCreated {
			t.Fatalf("entity type %q rejected: %d %s", typ, response.Code, response.Body.String())
		}
		definition := `{"entity_type":"` + typ + `","key":"key_` + strconv.Itoa(index) + `","label":"Field","value_type":"string","required":false}`
		if response := request(api, http.MethodPost, "/api/attribute-definitions", definition); response.Code != http.StatusCreated {
			t.Fatalf("attribute entity type %q rejected: %d %s", typ, response.Code, response.Body.String())
		}
	}

	for _, typ := range []string{"sensor", "person"} {
		entityBody := `{"name":"type-` + typ + `","type":"` + typ + `","status":"active","latitude":0,"longitude":0}`
		if response := request(api, http.MethodPost, "/api/entities", entityBody); response.Code != http.StatusBadRequest {
			t.Errorf("unsupported entity type %q status=%d body=%s", typ, response.Code, response.Body.String())
		}
		definitionBody := `{"entity_type":"` + typ + `","key":"invalid","label":"Field","value_type":"string","required":false}`
		if response := request(api, http.MethodPost, "/api/attribute-definitions", definitionBody); response.Code != http.StatusBadRequest {
			t.Errorf("unsupported attribute type %q status=%d body=%s", typ, response.Code, response.Body.String())
		}
		if response := request(api, http.MethodGet, "/api/attribute-definitions?entity_type="+typ, ""); response.Code != http.StatusBadRequest {
			t.Errorf("unsupported definition filter %q status=%d body=%s", typ, response.Code, response.Body.String())
		}
	}

	items, err := store.List(context.Background())
	if err != nil || len(items) != len(allowed) {
		t.Fatalf("unsupported types should not be persisted: count=%d err=%v", len(items), err)
	}
}

func TestAdvancedValidationLimitsAndImmutableHistory(t *testing.T) {
	api, _, _ := newTestAPI(t)
	created := request(api, http.MethodPost, "/api/entities", `{"name":"Typed","type":"vehicle","description":"","status":"active","latitude":0,"longitude":0}`)
	var entityEnvelope struct {
		Data entity.Entity `json:"data"`
	}
	if created.Code != 201 {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	_ = json.Unmarshal(created.Body.Bytes(), &entityEnvelope)
	id := entityEnvelope.Data.ID
	definition := `{"entity_type":"vehicle","key":"year","label":"Year","value_type":"number","required":true,"min_number":1900,"max_number":2100}`
	if response := request(api, http.MethodPost, "/api/attribute-definitions", definition); response.Code != 201 {
		t.Fatalf("definition=%d %s", response.Code, response.Body.String())
	}
	if response := request(api, http.MethodPost, "/api/attribute-definitions", definition); response.Code != 409 {
		t.Fatalf("duplicate definition=%d %s", response.Code, response.Body.String())
	}
	for _, body := range []string{`{"values":{"year":"2024"}}`, `{"values":{"year":2200}}`, `{"values":{"unknown":1}}`, `{"values":{}}`} {
		if response := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", body); response.Code != 400 {
			t.Fatalf("invalid attribute body %s -> %d", body, response.Code)
		}
	}
	if response := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", `{"values":{"year":2024}}`); response.Code != 200 {
		t.Fatalf("valid attributes=%d", response.Code)
	}
	var definitions struct {
		Data []repository.Definition `json:"data"`
	}
	response := request(api, http.MethodGet, "/api/attribute-definitions?entity_type=vehicle", "")
	if err := json.Unmarshal(response.Body.Bytes(), &definitions); err != nil || len(definitions.Data) != 1 {
		t.Fatalf("definitions response=%s err=%v", response.Body.String(), err)
	}
	did := definitions.Data[0].ID
	conflict := request(api, http.MethodPut, "/api/attribute-definitions/"+did, `{"label":"Year","required":true,"min_number":2050,"max_number":2100,"max_length":null,"archived":false}`)
	if conflict.Code != 409 {
		t.Fatalf("invalidating definition edit=%d %s", conflict.Code, conflict.Body.String())
	}
	archive := request(api, http.MethodPut, "/api/attribute-definitions/"+did, `{"label":"Year","required":false,"min_number":1900,"max_number":2100,"max_length":null,"archived":true}`)
	if archive.Code != 200 {
		t.Fatalf("archive=%d %s", archive.Code, archive.Body.String())
	}
	archivedWrite := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", `{"values":{"year":2024}}`)
	if archivedWrite.Code != 400 {
		t.Fatalf("archived definition write=%d", archivedWrite.Code)
	}
	clearActive := request(api, http.MethodPut, "/api/entities/"+id+"/attributes", `{"values":{}}`)
	if clearActive.Code != 200 {
		t.Fatalf("replace with archived value present=%d %s", clearActive.Code, clearActive.Body.String())
	}
	readArchived := request(api, http.MethodGet, "/api/entities/"+id+"/attributes", "")
	if !strings.Contains(readArchived.Body.String(), `"year":2024`) {
		t.Fatalf("archived value was deleted: %s", readArchived.Body.String())
	}
	nameConflict := request(api, http.MethodPost, "/api/tags", `{"name":"Fleet  Team"}`)
	if nameConflict.Code != 201 {
		t.Fatalf("tag create=%d", nameConflict.Code)
	}
	duplicate := request(api, http.MethodPost, "/api/tags", `{"name":" fleet team "}`)
	if duplicate.Code != 409 {
		t.Fatalf("normalized duplicate=%d %s", duplicate.Code, duplicate.Body.String())
	}
	for i := 0; i < 50; i++ {
		body := `{"entity_type":"other","key":"key_` + strconv.Itoa(i) + `","label":"Field","value_type":"boolean","required":false}`
		if response := request(api, http.MethodPost, "/api/attribute-definitions", body); response.Code != 201 {
			t.Fatalf("definition %d=%d %s", i, response.Code, response.Body.String())
		}
	}
	last := request(api, http.MethodPost, "/api/attribute-definitions", `{"entity_type":"other","key":"key_over","label":"Field","value_type":"boolean","required":false}`)
	if last.Code != 409 {
		t.Fatalf("definition cap=%d %s", last.Code, last.Body.String())
	}
	invalidCursor := request(api, http.MethodGet, "/api/entities/"+id+"/activity?cursor=not_base64", "")
	if invalidCursor.Code != 400 {
		t.Fatalf("invalid cursor=%d", invalidCursor.Code)
	}
	zeroLimit := request(api, http.MethodGet, "/api/entities/"+id+"/activity?limit=0", "")
	if zeroLimit.Code != 400 {
		t.Fatalf("zero limit=%d", zeroLimit.Code)
	}
	method := request(api, http.MethodDelete, "/api/entities/"+id+"/activity/00000000-0000-4000-8000-000000000001", "")
	if method.Code != 404 {
		t.Fatalf("unexpected event mutation route=%d", method.Code)
	}
}

func TestOversizedDescriptionIsPreservedAndActivitySnapshotIsBounded(t *testing.T) {
	api, store, _ := newTestAPI(t)
	large := strings.Repeat("x", 70<<10)
	body, _ := json.Marshal(map[string]any{"name": "Large", "type": "vehicle", "description": large, "status": "active", "latitude": 0, "longitude": 0})
	response := request(api, http.MethodPost, "/api/entities", string(body))
	if response.Code != http.StatusCreated {
		t.Fatalf("V1 create status changed for long description: %d", response.Code)
	}
	var envelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Description) != len(large) {
		t.Fatalf("entity description was altered: got %d bytes", len(envelope.Data.Description))
	}
	items, err := store.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("entity persistence: count=%d err=%v", len(items), err)
	}
	history := request(api, http.MethodGet, "/api/entities/"+envelope.Data.ID+"/activity", "")
	if history.Code != 200 || !strings.Contains(history.Body.String(), `"description_truncated":true`) {
		t.Fatalf("bounded event summary missing: %d %s", history.Code, history.Body.String())
	}
	if len(history.Body.Bytes()) > repository.MaxActivityPayload+2048 {
		t.Fatalf("activity response unexpectedly large: %d", len(history.Body.Bytes()))
	}
}
