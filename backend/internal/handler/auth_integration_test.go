package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoentity/backend/internal/auth"
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
