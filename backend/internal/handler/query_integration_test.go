package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"
)

func TestFullHTTPIntegrationLifecycleAndRestartPersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "http-lifecycle.db")
	ctx := context.Background()

	store, err := repository.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("initial store open: %v", err)
	}
	server := httptest.NewServer(New(store, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	client := server.Client()

	req := func(method, path, body string) (*http.Response, string) {
		t.Helper()
		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}
		r, err := http.NewRequest(method, server.URL+path, bodyReader)
		if err != nil {
			t.Fatalf("create request %s %s: %v", method, path, err)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatalf("execute request %s %s: %v", method, path, err)
		}
		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read response %s %s: %v", method, path, err)
		}
		return resp, string(respBytes)
	}

	// 1. Health
	resp, body := req(http.MethodGet, "/api/health", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("1. health failed: %d %s", resp.StatusCode, body)
	}

	// 2. Legacy V1 entity list (empty)
	resp, body = req(http.MethodGet, "/api/entities", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"data":[]`) || strings.Contains(body, `"meta"`) {
		t.Fatalf("2. empty legacy list failed: %d %s", resp.StatusCode, body)
	}

	// 10. Entity create (create 3 entities via V1 POST)
	e1Body := `{"name":"Alpha Truck","type":"vehicle","description":"Central depot depot-01","status":"active","latitude":-6.2,"longitude":106.8}`
	resp, body = req(http.MethodPost, "/api/entities", e1Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("10. create entity 1 failed: %d %s", resp.StatusCode, body)
	}
	var e1Envelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &e1Envelope); err != nil {
		t.Fatal(err)
	}
	e1ID := e1Envelope.Data.ID

	e2Body := `{"name":"Beta Van","type":"vehicle","description":"South station","status":"inactive","latitude":-6.3,"longitude":106.9}`
	resp, body = req(http.MethodPost, "/api/entities", e2Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("10. create entity 2 failed: %d %s", resp.StatusCode, body)
	}
	var e2Envelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &e2Envelope); err != nil {
		t.Fatal(err)
	}
	e2ID := e2Envelope.Data.ID

	e3Body := `{"name":"Gamma Facility","type":"facility","description":"Main factory depot-02","status":"active","latitude":-6.4,"longitude":106.7}`
	resp, body = req(http.MethodPost, "/api/entities", e3Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("10. create entity 3 failed: %d %s", resp.StatusCode, body)
	}
	var e3Envelope struct {
		Data entity.Entity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &e3Envelope); err != nil {
		t.Fatal(err)
	}
	e3ID := e3Envelope.Data.ID

	// 15. Metadata preservation: add attribute definition, custom attribute, tag, category, and verify activity
	defBody := `{"entity_type":"vehicle","key":"vin","label":"Vehicle VIN","value_type":"string","required":false,"max_length":50}`
	resp, body = req(http.MethodPost, "/api/attribute-definitions", defBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("15. create definition failed: %d %s", resp.StatusCode, body)
	}

	attrValBody := `{"values":{"vin":"VIN-ALPHA-1234"}}`
	resp, body = req(http.MethodPut, "/api/entities/"+e1ID+"/attributes", attrValBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("15. put attributes failed: %d %s", resp.StatusCode, body)
	}

	tagBody := `{"name":"Fleet-1"}`
	resp, body = req(http.MethodPost, "/api/tags", tagBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("15. create tag failed: %d %s", resp.StatusCode, body)
	}
	var tagEnvelope struct {
		Data repository.CatalogItem `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &tagEnvelope); err != nil {
		t.Fatal(err)
	}

	assignTagBody, _ := json.Marshal(map[string]any{"tag_ids": []string{tagEnvelope.Data.ID}})
	resp, body = req(http.MethodPut, "/api/entities/"+e1ID+"/tags", string(assignTagBody))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("15. assign tag failed: %d %s", resp.StatusCode, body)
	}

	// 2. Legacy V1 entity list (unpaginated, returns all 3 entities, no "meta")
	resp, body = req(http.MethodGet, "/api/entities", "")
	if resp.StatusCode != http.StatusOK || strings.Contains(body, `"meta"`) {
		t.Fatalf("2. legacy list failed: %d %s", resp.StatusCode, body)
	}
	var legacyList struct {
		Data []entity.Entity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &legacyList); err != nil || len(legacyList.Data) != 3 {
		t.Fatalf("2. legacy list count != 3: %+v", legacyList)
	}

	// 3. New paginated entity endpoint
	resp, body = req(http.MethodGet, "/api/entities/query?limit=2", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("3. query endpoint failed: %d %s", resp.StatusCode, body)
	}
	var page1 struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			Limit         int            `json:"limit"`
			NextCursor    string         `json:"next_cursor"`
			MatchingCount int            `json:"matching_count"`
			StatusCounts  map[string]int `json:"status_counts"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Data) != 2 || page1.Meta.MatchingCount != 3 || page1.Meta.NextCursor == "" {
		t.Fatalf("3. query page 1 unexpected: %+v", page1)
	}
	if page1.Meta.StatusCounts["active"] != 2 || page1.Meta.StatusCounts["inactive"] != 1 {
		t.Fatalf("3. status counts unexpected: %+v", page1.Meta.StatusCounts)
	}

	// 4. Cursor continuation
	resp, body = req(http.MethodGet, "/api/entities/query?limit=2&cursor="+url.QueryEscape(page1.Meta.NextCursor), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("4. cursor continuation failed: %d %s", resp.StatusCode, body)
	}
	var page2 struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			Limit         int    `json:"limit"`
			NextCursor    string `json:"next_cursor"`
			MatchingCount int    `json:"matching_count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Data) != 1 || page2.Meta.NextCursor != "" {
		t.Fatalf("4. query page 2 unexpected: %+v", page2)
	}
	if page2.Data[0].ID != e3ID {
		t.Fatalf("4. page 2 expected e3ID %s, got %s", e3ID, page2.Data[0].ID)
	}

	// 5. Invalid cursor
	resp, body = req(http.MethodGet, "/api/entities/query?cursor=invalid-cursor-token", "")
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"INVALID_QUERY"`) {
		t.Fatalf("5. invalid cursor expected 400 INVALID_QUERY: %d %s", resp.StatusCode, body)
	}

	// Filter-mismatched cursor
	resp, body = req(http.MethodGet, "/api/entities/query?type=vehicle&cursor="+url.QueryEscape(page1.Meta.NextCursor), "")
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"INVALID_QUERY"`) {
		t.Fatalf("5. filter-mismatched cursor expected 400 INVALID_QUERY: %d %s", resp.StatusCode, body)
	}

	// 6. Search (literal substring in name and description)
	resp, body = req(http.MethodGet, "/api/entities/query?q=depot-01", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("6. search failed: %d %s", resp.StatusCode, body)
	}
	var searchResp struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			MatchingCount int `json:"matching_count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &searchResp); err != nil {
		t.Fatal(err)
	}
	if len(searchResp.Data) != 1 || searchResp.Data[0].ID != e1ID || searchResp.Meta.MatchingCount != 1 {
		t.Fatalf("6. search result unexpected: %+v", searchResp)
	}

	// 7. Search combined with filters (q=depot & type=vehicle & status=active)
	resp, body = req(http.MethodGet, "/api/entities/query?q=depot&type=vehicle&status=active", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("7. search combined failed: %d %s", resp.StatusCode, body)
	}
	var combinedResp struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			MatchingCount int            `json:"matching_count"`
			StatusCounts  map[string]int `json:"status_counts"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &combinedResp); err != nil {
		t.Fatal(err)
	}
	if len(combinedResp.Data) != 1 || combinedResp.Data[0].ID != e1ID {
		t.Fatalf("7. search combined result unexpected: %+v", combinedResp)
	}
	// Under q=depot and type=vehicle: active=1, inactive=0 (since Beta Van has no 'depot'), maintenance=0
	if combinedResp.Meta.StatusCounts["active"] != 1 || combinedResp.Meta.StatusCounts["inactive"] != 0 {
		t.Fatalf("7. status counts under q and type unexpected: %+v", combinedResp.Meta.StatusCounts)
	}

	// 8. Short-query behavior (1 or 2 character fallback: q=Al)
	resp, body = req(http.MethodGet, "/api/entities/query?q=Al", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("8. short query failed: %d %s", resp.StatusCode, body)
	}
	var shortResp struct {
		Data []entity.Entity `json:"data"`
		Meta struct {
			MatchingCount int `json:"matching_count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &shortResp); err != nil {
		t.Fatal(err)
	}
	if len(shortResp.Data) != 1 || shortResp.Data[0].ID != e1ID {
		t.Fatalf("8. short query result unexpected: %+v", shortResp)
	}

	// 9. Map dataset endpoint and extent
	resp, body = req(http.MethodGet, "/api/entities/map?west=106&south=-7&east=107&north=-6", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("9. map entities failed: %d %s", resp.StatusCode, body)
	}
	var mapResp struct {
		Data []repository.MapEntity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &mapResp); err != nil {
		t.Fatal(err)
	}
	if len(mapResp.Data) != 3 {
		t.Fatalf("9. map entities expected 3, got %d", len(mapResp.Data))
	}

	resp, body = req(http.MethodGet, "/api/entities/map/extent?type=vehicle", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("9. map extent failed: %d %s", resp.StatusCode, body)
	}
	var extentResp struct {
		Data repository.MapExtent `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &extentResp); err != nil {
		t.Fatal(err)
	}
	if extentResp.Data.Count != 2 || extentResp.Data.Bounds == nil {
		t.Fatalf("9. map extent unexpected: %+v", extentResp)
	}

	// 11. Entity rename (via V1 PUT)
	renameBody := `{"name":"Alpha SuperTruck","type":"vehicle","description":"Renamed description","status":"active","latitude":-6.2,"longitude":106.8}`
	resp, body = req(http.MethodPut, "/api/entities/"+e1ID, renameBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("11. rename entity failed: %d %s", resp.StatusCode, body)
	}

	// 12. Search index synchronization: old name no longer matches, new name matches
	resp, body = req(http.MethodGet, "/api/entities/query?q=Alpha%20Truck", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("12. search old name failed: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, e1ID) {
		t.Fatalf("12. old name still matches: %s", body)
	}

	resp, body = req(http.MethodGet, "/api/entities/query?q=SuperTruck", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("12. search new name failed: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, e1ID) {
		t.Fatalf("12. new name did not match: %s", body)
	}

	// 13. Entity deletion (delete e2 via V1 DELETE)
	resp, body = req(http.MethodDelete, "/api/entities/"+e2ID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("13. delete entity failed: %d %s", resp.StatusCode, body)
	}

	// 14. Search exclusion after deletion
	resp, body = req(http.MethodGet, "/api/entities/query?q=Beta%20Van", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("14. search deleted entity failed: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, e2ID) {
		t.Fatalf("14. deleted entity still matches: %s", body)
	}

	// Verify activity history after deletion remains readable
	resp, body = req(http.MethodGet, "/api/entities/"+e2ID+"/activity", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"entity.deleted"`) {
		t.Fatalf("14. activity history after deletion failed: %d %s", resp.StatusCode, body)
	}

	// Close server and database to prepare for restart verification
	server.Close()
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	// 16. Restart persistence & 17. Migration rerun behavior
	reopenedStore, err := repository.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("16/17. reopened store: %v", err)
	}
	defer reopenedStore.Close()

	restartedServer := httptest.NewServer(New(reopenedStore, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	defer restartedServer.Close()
	server = restartedServer

	// Verify health
	resp, body = req(http.MethodGet, "/api/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("16. post-restart health: %d %s", resp.StatusCode, body)
	}

	// Verify entity count (e1 and e3 remain, e2 is gone)
	resp, body = req(http.MethodGet, "/api/entities", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("16. post-restart V1 list: %d %s", resp.StatusCode, body)
	}
	var postRestartList struct {
		Data []entity.Entity `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &postRestartList); err != nil || len(postRestartList.Data) != 2 {
		t.Fatalf("16. post-restart entity count != 2: %+v", postRestartList)
	}

	// Verify search works on post-restart server
	resp, body = req(http.MethodGet, "/api/entities/query?q=SuperTruck", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, e1ID) {
		t.Fatalf("16. post-restart search failed: %d %s", resp.StatusCode, body)
	}

	// Verify metadata persisted (custom attributes and tags)
	resp, body = req(http.MethodGet, "/api/entities/"+e1ID+"/attributes", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "VIN-ALPHA-1234") {
		t.Fatalf("16. post-restart attributes failed: %d %s", resp.StatusCode, body)
	}
	resp, body = req(http.MethodGet, "/api/entities/"+e1ID+"/tags", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "Fleet-1") {
		t.Fatalf("16. post-restart tags failed: %d %s", resp.StatusCode, body)
	}

	// Verify deleted entity activity events persisted
	resp, body = req(http.MethodGet, "/api/entities/"+e2ID+"/activity", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"entity.deleted"`) {
		t.Fatalf("16. post-restart deleted entity activity failed: %d %s", resp.StatusCode, body)
	}
}
