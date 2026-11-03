package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/monitoring"
	"time"
)

func TestComposedProductionAuthFlowAllowsFrontendCRUDAndAudit(t *testing.T) {
	api, store, _ := newTestAPI(t)
	ctx := context.Background()
	svc, err := auth.New(store.DB(), auth.Config{PublicOrigin: "https://geo.test", CookieSecure: true})
	if err != nil {
		t.Fatal(err)
	}
	passphrase := "amber river cedar meadow lantern harbor"
	if err = svc.Provision(ctx, passphrase); err != nil {
		t.Fatal(err)
	}
	production := svc.Middleware(api)
	loginBody, _ := json.Marshal(map[string]string{"password": passphrase})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Origin", "https://geo.test")
	loginRes := httptest.NewRecorder()
	production.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.Code, loginRes.Body.String())
	}
	var login struct {
		CSRF string `json:"csrfToken"`
	}
	if err = json.Unmarshal(loginRes.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, c := range loginRes.Result().Cookies() {
		if c.Name == "__Host-geoentity_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie missing")
	}
	unauth := request(production, http.MethodGet, "/api/entities", "")
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status=%d", unauth.Code)
	}
	readReq := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	readReq.AddCookie(cookie)
	readRes := httptest.NewRecorder()
	production.ServeHTTP(readRes, readReq)
	if readRes.Code != http.StatusOK {
		t.Fatalf("authenticated list status=%d body=%s", readRes.Code, readRes.Body.String())
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/entities", bytes.NewBufferString(`{"name":"Protected write","type":"facility","description":"","status":"active","latitude":1,"longitude":2}`))
	createReq.AddCookie(cookie)
	createReq.Header.Set("Origin", "https://geo.test")
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", login.CSRF)
	createRes := httptest.NewRecorder()
	production.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("authenticated create status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	var audits int
	if err = store.DB().QueryRow(`SELECT count(*) FROM admin_audit_events WHERE action='entity.created' AND actor_id='1'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("attributed entity audit count=%d err=%v", audits, err)
	}
}

func TestDeviceCredentialRouteSplitAndStrictIngestValidation(t *testing.T) {
	api, store, _ := newTestAPI(t)
	ctx := context.Background()
	svc, err := auth.New(store.DB(), auth.Config{PublicOrigin: "https://geo.test", CookieSecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Provision(ctx, "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	entityID := "10000000-0000-4000-8000-000000000011"
	if err = store.Create(ctx, entity.Entity{ID: entityID, Name: "Device host", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	_, credential, err := monitoring.New(store.DB()).CreateCredential(ctx, entityID, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	production := svc.Middleware(api)
	req := httptest.NewRequest(http.MethodPost, "/api/device/v1/observations", bytes.NewBufferString(`{"sensor_id":"temp","unit":"Cel","observed_at":"2026-11-03T00:00:00Z"}`))
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "one")
	res := httptest.NewRecorder()
	production.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("missing value status=%d body=%s", res.Code, res.Body.String())
	}
	adminReq := httptest.NewRequest(http.MethodGet, "/api/admin/devices", nil)
	adminReq.Header.Set("Authorization", "Bearer "+credential)
	adminRes := httptest.NewRecorder()
	production.ServeHTTP(adminRes, adminReq)
	if adminRes.Code != http.StatusUnauthorized {
		t.Fatalf("device credential accessed admin API: %d", adminRes.Code)
	}
	unknown := httptest.NewRequest(http.MethodGet, "/api/not-a-route", nil)
	unknownRes := httptest.NewRecorder()
	production.ServeHTTP(unknownRes, unknown)
	if unknownRes.Code != http.StatusUnauthorized {
		t.Fatalf("unknown API path escaped admin auth: %d", unknownRes.Code)
	}
	adminCookie, _, err := svc.Login(ctx, "127.0.0.1", "amber river cedar meadow lantern harbor")
	if err != nil {
		t.Fatal(err)
	}
	cookieOnly := httptest.NewRequest(http.MethodPost, "/api/device/v1/observations", bytes.NewBufferString(`{}`))
	cookieOnly.AddCookie(&http.Cookie{Name: "__Host-geoentity_session", Value: adminCookie})
	cookieOnly.Header.Set("Content-Type", "application/json")
	cookieOnly.Header.Set("Idempotency-Key", "one")
	cookieRes := httptest.NewRecorder()
	production.ServeHTTP(cookieRes, cookieOnly)
	if cookieRes.Code != http.StatusUnauthorized {
		t.Fatalf("admin cookie authenticated device route: %d", cookieRes.Code)
	}
}
