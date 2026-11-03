package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"geoentity/backend/internal/geofence"
)

const circleBody = `{"name":"Depot","enabled":true,"geometry":{"type":"Circle","center":[106.8,-6.2],"radius_m":250}}`
const polygonBody = `{"name":"Yard","enabled":false,"geometry":{"type":"Polygon","coordinates":[[[106.7,-6.3],[106.9,-6.3],[106.9,-6.1],[106.7,-6.1],[106.7,-6.3]]]}}`

func TestGeofenceHTTPCRUDAndExistingEntityAPI(t *testing.T) {
	api, _, _ := newTestAPI(t)
	created := request(api, http.MethodPost, "/api/geofences", circleBody)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var envelope struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Enabled  bool   `json:"enabled"`
			Geometry struct {
				Type    string    `json:"type"`
				Center  []float64 `json:"center"`
				RadiusM float64   `json:"radius_m"`
			} `json:"geometry"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	item := envelope.Data
	if item.ID == "" || item.Name != "Depot" || !item.Enabled || item.Geometry.Type != "Circle" || item.Geometry.Center[0] != 106.8 || item.Geometry.Center[1] != -6.2 || item.Geometry.RadiusM != 250 {
		t.Fatalf("created geofence=%+v", item)
	}

	detail := request(api, http.MethodGet, "/api/geofences/"+item.ID, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"type":"Circle"`) {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	updated := request(api, http.MethodPut, "/api/geofences/"+item.ID, polygonBody)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"type":"Polygon"`) || !strings.Contains(updated.Body.String(), `"created_at":"`+item.CreatedAt+`"`) {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	list := request(api, http.MethodGet, "/api/geofences?enabled=false&limit=1", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"name":"Yard"`) {
		t.Fatalf("list filter: %d %s", list.Code, list.Body.String())
	}

	entities := request(api, http.MethodGet, "/api/entities", "")
	if entities.Code != http.StatusOK || strings.TrimSpace(entities.Body.String()) != `{"data":[]}` {
		t.Fatalf("V1 list changed: %d %s", entities.Code, entities.Body.String())
	}
	deleted := request(api, http.MethodDelete, "/api/geofences/"+item.ID, "")
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
	missing := request(api, http.MethodGet, "/api/geofences/"+item.ID, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing detail: %d %s", missing.Code, missing.Body.String())
	}
}

func TestGeofenceHTTPValidationErrors(t *testing.T) {
	api, _, _ := newTestAPI(t)
	for _, test := range []struct {
		name string
		body string
		code string
	}{
		{"missing enabled", `{"name":"x","geometry":{"type":"Circle","center":[0,0],"radius_m":2}}`, "VALIDATION_ERROR"},
		{"circle radius zero", `{"name":"x","enabled":true,"geometry":{"type":"Circle","center":[0,0],"radius_m":0}}`, "VALIDATION_ERROR"},
		{"open polygon", `{"name":"x","enabled":true,"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,1]]]}}`, "VALIDATION_ERROR"},
		{"unknown root", `{"name":"x","enabled":true,"geometry":{"type":"Circle","center":[0,0],"radius_m":1},"extra":true}`, "INVALID_JSON"},
		{"unknown geometry", `{"name":"x","enabled":true,"geometry":{"type":"Circle","center":[0,0],"radius_m":1,"buffer":true}}`, "VALIDATION_ERROR"},
		{"missing geometry", `{"name":"x","enabled":true}`, "VALIDATION_ERROR"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(api, http.MethodPost, "/api/geofences", test.body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/api/geofences", strings.NewReader(circleBody))
	req.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	api.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"INVALID_JSON"`) {
		t.Fatalf("content type response: %d %s", response.Code, response.Body.String())
	}
	large := `{"name":"` + strings.Repeat("x", (1<<20)+1) + `","enabled":true,"geometry":{"type":"Circle","center":[0,0],"radius_m":1}}`
	oversized := request(api, http.MethodPost, "/api/geofences", large)
	if oversized.Code != http.StatusRequestEntityTooLarge || !strings.Contains(oversized.Body.String(), `"code":"REQUEST_TOO_LARGE"`) {
		t.Fatalf("oversized body response: %d %s", oversized.Code, oversized.Body.String())
	}
}

func TestGeofenceHTTPConflictIDsAndStrictQueries(t *testing.T) {
	api, _, _ := newTestAPI(t)
	first := request(api, http.MethodPost, "/api/geofences", circleBody)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	duplicate := strings.Replace(circleBody, "Depot", " depot ", 1)
	if response := request(api, http.MethodPost, "/api/geofences", duplicate); response.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"/api/geofences/not-a-uuid", "/api/geofences?unknown=x", "/api/geofences?enabled=maybe", "/api/geofences?limit=101"} {
		response := request(api, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestGeofenceHTTPActiveDefinitionLimit(t *testing.T) {
	api, _, _ := newTestAPI(t)
	for i := 0; i <= geofence.MaxActiveGeofences; i++ {
		body := strings.Replace(circleBody, "Depot", fmt.Sprintf("Depot %03d", i), 1)
		response := request(api, http.MethodPost, "/api/geofences", body)
		if i < geofence.MaxActiveGeofences && response.Code != http.StatusCreated {
			t.Fatalf("create %d: %d %s", i, response.Code, response.Body.String())
		}
		if i == geofence.MaxActiveGeofences && (response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"GEOFENCE_LIMIT"`)) {
			t.Fatalf("limit response: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestGeofenceMembershipAndEventHTTPContracts(t *testing.T) {
	api, _, _ := newTestAPI(t)
	createdFence := request(api, http.MethodPost, "/api/geofences", circleBody)
	if createdFence.Code != http.StatusCreated {
		t.Fatalf("create geofence: %d %s", createdFence.Code, createdFence.Body.String())
	}
	var fenceEnvelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createdFence.Body.Bytes(), &fenceEnvelope); err != nil {
		t.Fatal(err)
	}
	createdEntity := request(api, http.MethodPost, "/api/entities", `{"name":"Mover","type":"facility","description":"","status":"active","latitude":-6.2,"longitude":106.81}`)
	if createdEntity.Code != http.StatusCreated {
		t.Fatalf("create entity: %d %s", createdEntity.Code, createdEntity.Body.String())
	}
	var entityEnvelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createdEntity.Body.Bytes(), &entityEnvelope); err != nil {
		t.Fatal(err)
	}
	entityID := entityEnvelope.Data.ID
	memberships := request(api, http.MethodGet, "/api/entities/"+entityID+"/geofences", "")
	if memberships.Code != http.StatusOK || !strings.Contains(memberships.Body.String(), `"state":"outside"`) {
		t.Fatalf("membership read: %d %s", memberships.Code, memberships.Body.String())
	}
	update := request(api, http.MethodPut, "/api/entities/"+entityID, `{"name":"Mover","type":"facility","description":"","status":"active","latitude":-6.2,"longitude":106.8}`)
	if update.Code != http.StatusOK {
		t.Fatalf("coordinate update: %d %s", update.Code, update.Body.String())
	}
	update = request(api, http.MethodPut, "/api/entities/"+entityID, `{"name":"Mover","type":"facility","description":"","status":"active","latitude":-6.2,"longitude":106.81}`)
	if update.Code != http.StatusOK {
		t.Fatalf("coordinate exit: %d %s", update.Code, update.Body.String())
	}
	if repeated := request(api, http.MethodPut, "/api/entities/"+entityID, `{"name":"Mover","type":"facility","description":"","status":"active","latitude":-6.2,"longitude":106.8}`); repeated.Code != http.StatusOK {
		t.Fatalf("identical coordinate retry: %d %s", repeated.Code, repeated.Body.String())
	}
	events := request(api, http.MethodGet, "/api/geofences/"+fenceEnvelope.Data.ID+"/events?limit=10", "")
	if events.Code != http.StatusOK || !strings.Contains(events.Body.String(), `"event_type":"enter"`) || !strings.Contains(events.Body.String(), `"event_type":"exit"`) || !strings.Contains(events.Body.String(), `"source":"coordinate_update"`) {
		t.Fatalf("event history: %d %s", events.Code, events.Body.String())
	}
	for _, path := range []string{
		"/api/geofences/" + fenceEnvelope.Data.ID + "/events?limit=101",
		"/api/geofences/" + fenceEnvelope.Data.ID + "/events?from=tomorrow",
		"/api/geofences/" + fenceEnvelope.Data.ID + "/events?from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z",
		"/api/geofences/" + fenceEnvelope.Data.ID + "/events?entity_id=bad",
		"/api/geofences/" + fenceEnvelope.Data.ID + "/events?unknown=x",
		"/api/entities/bad/geofences",
		"/api/entities/" + entityID + "/geofences?unknown=x",
	} {
		response := request(api, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	deleted := request(api, http.MethodDelete, "/api/geofences/"+fenceEnvelope.Data.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete geofence: %d %s", deleted.Code, deleted.Body.String())
	}
	archivedEvents := request(api, http.MethodGet, "/api/geofences/"+fenceEnvelope.Data.ID+"/events", "")
	if archivedEvents.Code != http.StatusOK || !strings.Contains(archivedEvents.Body.String(), `"geofence_name":"Depot"`) {
		t.Fatalf("deleted geofence history: %d %s", archivedEvents.Code, archivedEvents.Body.String())
	}
	if repeatedDelete := request(api, http.MethodDelete, "/api/geofences/"+fenceEnvelope.Data.ID, ""); repeatedDelete.Code != http.StatusNotFound || !strings.Contains(repeatedDelete.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("repeated geofence delete: %d %s", repeatedDelete.Code, repeatedDelete.Body.String())
	}
}
