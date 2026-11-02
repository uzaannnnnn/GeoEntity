package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"
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
