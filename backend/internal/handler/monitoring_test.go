package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/monitoring"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeviceLimiterEnforcesAndBoundsBuckets(t *testing.T) {
	deviceLimits.Lock()
	deviceLimits.buckets = map[string]limitBucket{}
	deviceLimits.ops = 0
	now := time.Now()
	for i := 0; i < 20001; i++ {
		deviceLimits.buckets["stale-"+string(rune(i+1))] = limitBucket{start: now.Add(-3 * time.Minute)}
	}
	deviceLimits.Unlock()
	if !takeLimit("fresh", 1, now) {
		t.Fatal("first request rejected")
	}
	deviceLimits.Lock()
	size := len(deviceLimits.buckets)
	deviceLimits.Unlock()
	if size > 20000 {
		t.Fatalf("limiter buckets=%d", size)
	}
	if takeLimit("fresh", 1, now.Add(time.Second)) {
		t.Fatal("per-key request limit not enforced")
	}
}

func TestDeviceRateLimitIPTrustsOnlyConfiguredProxy(t *testing.T) {
	api := &API{trustedProxyCIDRs: []string{"192.0.2.0/24"}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "198.51.100.20:9000"
	req.Header.Set("X-Real-IP", "203.0.113.7")
	if got := api.clientIP(req); got != "198.51.100.20" {
		t.Fatalf("spoofed forwarded address accepted: %s", got)
	}
	req.RemoteAddr = "192.0.2.10:9000"
	if got := api.clientIP(req); got != "203.0.113.7" {
		t.Fatalf("trusted proxy client address=%s", got)
	}
}

func TestMonitoringHistoryPaginationAndLatest(t *testing.T) {
	api, store, _ := newTestAPI(t)
	ctx := context.Background()
	entityID := "10000000-0000-4000-8000-000000000021"
	if err := store.Create(ctx, entity.Entity{ID: entityID, Name: "Monitor target", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	svc := monitoring.New(store.DB())
	_, token, err := svc.CreateCredential(ctx, entityID, "sensor", nil)
	if err != nil {
		t.Fatal(err)
	}
	device, boundEntity, credentialID, err := svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, _, err = svc.Ingest(ctx, device, boundEntity, credentialID, fmt.Sprintf("key-%d", i), monitoring.Input{SensorID: "temperature", Value: float64(i), Unit: "Cel", ObservedAt: time.Now().UTC().Add(-time.Duration(i) * time.Second).Format(time.RFC3339Nano)})
		if err != nil {
			t.Fatal(err)
		}
	}
	first := httptest.NewRecorder()
	api.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/entities/"+entityID+"/sensors/observations?limit=1", nil))
	if first.Code != 200 {
		t.Fatalf("first history page status=%d body=%s", first.Code, first.Body.String())
	}
	var page struct {
		Data   []map[string]any `json:"data"`
		Cursor string           `json:"next_cursor"`
	}
	if err = json.Unmarshal(first.Body.Bytes(), &page); err != nil || len(page.Data) != 1 || page.Cursor == "" {
		t.Fatalf("first page=%+v err=%v", page, err)
	}
	firstID := page.Data[0]["id"]
	second := httptest.NewRecorder()
	api.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/entities/"+entityID+"/sensors/observations?limit=1&cursor="+page.Cursor, nil))
	if second.Code != 200 {
		t.Fatalf("second history page status=%d body=%s", second.Code, second.Body.String())
	}
	var page2 struct {
		Data   []map[string]any `json:"data"`
		Cursor string           `json:"next_cursor"`
	}
	if err = json.Unmarshal(second.Body.Bytes(), &page2); err != nil || len(page2.Data) != 1 || page2.Data[0]["id"] == firstID {
		t.Fatalf("second page=%+v err=%v", page2, err)
	}
	latest := httptest.NewRecorder()
	api.ServeHTTP(latest, httptest.NewRequest(http.MethodGet, "/api/monitoring/latest?entity_id="+entityID, nil))
	if latest.Code != 200 {
		t.Fatalf("latest status=%d body=%s", latest.Code, latest.Body.String())
	}
	var latestPage struct {
		Data []map[string]any `json:"data"`
	}
	if err = json.Unmarshal(latest.Body.Bytes(), &latestPage); err != nil || len(latestPage.Data) != 1 {
		t.Fatalf("latest readings=%+v err=%v", latestPage, err)
	}
}
