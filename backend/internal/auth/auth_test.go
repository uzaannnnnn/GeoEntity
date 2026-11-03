package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/repository"
)

func testService(t *testing.T) (*auth.Service, *sql.DB) {
	t.Helper()
	store, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(store.DB(), auth.Config{PublicOrigin: "https://geo.test", CookieSecure: true})
	if err != nil {
		t.Fatal(err)
	}
	return svc, store.DB()
}

func TestProvisionNormalizationAndOnlyHashedSessionToken(t *testing.T) {
	svc, db := testService(t)
	ctx := context.Background()
	if exists, err := svc.HasAdmin(ctx); err != nil || exists {
		t.Fatalf("unexpected default admin: exists=%v err=%v", exists, err)
	}
	passwordNFD := strings.Repeat("e\u0301", 16)
	if err := svc.Provision(ctx, passwordNFD); err != nil {
		t.Fatal(err)
	}
	var verifier string
	if err := db.QueryRow(`SELECT password_verifier FROM admin_users WHERE id=1`).Scan(&verifier); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(verifier, passwordNFD) {
		t.Fatal("password appears in stored verifier")
	}
	if !strings.HasPrefix(verifier, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected Argon2id parameters: %q", verifier)
	}
	token, _, err := svc.Login(ctx, "127.0.0.1", strings.Repeat("é", 16))
	if err != nil {
		t.Fatal("NFC-equivalent password did not verify:", err)
	}
	h := sha256.Sum256([]byte(token))
	var stored string
	if err = db.QueryRow(`SELECT token_hash FROM web_sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != hex.EncodeToString(h[:]) || stored == token {
		t.Fatalf("session token was not stored only as a hash: %q", stored)
	}
}

func TestPasswordMinimumAndBlocklist(t *testing.T) {
	for _, tc := range []struct {
		password string
		ok       bool
	}{{strings.Repeat("a", 14), false}, {strings.Repeat("a", 15), true}, {"correcthorsebatterystaple", false}, {strings.Repeat("🔐", 15), true}} {
		if err := auth.ValidatePassword(tc.password); (err == nil) != tc.ok {
			t.Errorf("ValidatePassword(%q) err=%v", tc.password, err)
		}
	}
}

func TestMiddlewareAuthenticationCSRFOriginLogoutAndSessionExpiry(t *testing.T) {
	svc, db := testService(t)
	ctx := context.Background()
	if err := svc.Provision(ctx, "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	wrapped := svc.Middleware(base)
	unauth := httptest.NewRecorder()
	wrapped.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/entities", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated route status=%d", unauth.Code)
	}
	health := httptest.NewRecorder()
	wrapped.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusNoContent {
		t.Fatalf("health should remain public: %d", health.Code)
	}
	body, _ := json.Marshal(map[string]string{"password": "amber river cedar meadow lantern harbor"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Origin", "https://geo.test")
	login := httptest.NewRecorder()
	wrapped.ServeHTTP(login, loginReq)
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var loginBody struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil || len(loginBody.CSRF) != 64 {
		t.Fatalf("invalid login response: %s err=%v", login.Body.String(), err)
	}
	var cookie *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == "__Host-geoentity_session" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("insecure session cookie: %#v", cookie)
	}
	request := func(origin, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/entities", strings.NewReader(`{}`))
		req.AddCookie(cookie)
		req.Header.Set("Origin", origin)
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
		return rr
	}
	if got := request("https://geo.test", loginBody.CSRF).Code; got != http.StatusNoContent {
		t.Fatalf("valid CSRF request status=%d", got)
	}
	if got := request("https://geo.test", "").Code; got != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", got)
	}
	if got := request("https://evil.test", loginBody.CSRF).Code; got != http.StatusForbidden {
		t.Fatalf("untrusted Origin status=%d", got)
	}
	if _, err := db.Exec(`UPDATE web_sessions SET last_seen_at=?`, time.Now().Add(-auth.IdleTimeout-time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	expired := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	expired.AddCookie(cookie)
	response := httptest.NewRecorder()
	wrapped.ServeHTTP(response, expired)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status=%d", response.Code)
	}
}

func TestPasswordRotationRevokesSessions(t *testing.T) {
	svc, db := testService(t)
	ctx := context.Background()
	if err := svc.Provision(ctx, "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, "127.0.0.1", "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RotatePassword(ctx, "silver mountain birch ocean candle field"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM web_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("sessions after recovery=%d err=%v", count, err)
	}
	if _, _, err := svc.Login(ctx, "127.0.0.1", "silver mountain birch ocean candle field"); err != nil {
		t.Fatalf("new password failed: %v", err)
	}
	if _, _, err := svc.Login(ctx, "127.0.0.1", "amber river cedar meadow lantern harbor"); err == nil {
		t.Fatal("old password remained valid")
	}
}

func TestDefaultDenyProtectsEveryExistingDataRoute(t *testing.T) {
	svc, _ := testService(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	wrapped := svc.Middleware(next)
	for _, path := range []string{"/api/entities", "/api/entities/10000000-0000-4000-8000-000000000001", "/api/entities/query", "/api/entities/map", "/api/attribute-definitions", "/api/tags", "/api/categories", "/api/imports/csv/preview", "/api/imports/csv/commit", "/api/exports/entities.csv", "/api/geofences", "/api/entities/10000000-0000-4000-8000-000000000001/geofences", "/api/admin/audit", "/api/not-a-route"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			req := httptest.NewRequest(method, path, nil)
			response := httptest.NewRecorder()
			wrapped.ServeHTTP(response, req)
			if response.Code != http.StatusUnauthorized {
				t.Errorf("%s %s status=%d, want 401", method, path, response.Code)
			}
		}
	}
}

func TestRefererFallbackAndSessionBootstrapOriginGuard(t *testing.T) {
	svc, _ := testService(t)
	if err := svc.Provision(context.Background(), "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	wrapped := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	body := bytes.NewBufferString(`{"password":"amber river cedar meadow lantern harbor"}`)
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	login.Header.Set("Referer", "https://geo.test/sign-in")
	response := httptest.NewRecorder()
	wrapped.ServeHTTP(response, login)
	if response.Code != http.StatusOK {
		t.Fatalf("trusted Referer login status=%d", response.Code)
	}
	cookie := response.Result().Cookies()[0]
	bootstrap := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	bootstrap.AddCookie(cookie)
	blocked := httptest.NewRecorder()
	wrapped.ServeHTTP(blocked, bootstrap)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("session bootstrap without Origin/Referer status=%d", blocked.Code)
	}
	bootstrap.Header.Set("Referer", "https://geo.test/")
	allowed := httptest.NewRecorder()
	wrapped.ServeHTTP(allowed, bootstrap)
	if allowed.Code != http.StatusOK || !bytes.Contains(allowed.Body.Bytes(), []byte(`"authenticated":true`)) {
		t.Fatalf("trusted session bootstrap: %d %s", allowed.Code, allowed.Body.String())
	}
}

func TestLoginSessionRotationLogoutAndAbsoluteExpiry(t *testing.T) {
	svc, db := testService(t)
	ctx, password := context.Background(), "amber river cedar meadow lantern harbor"
	if err := svc.Provision(ctx, password); err != nil {
		t.Fatal(err)
	}
	wrapped := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	type loginResult struct {
		cookie *http.Cookie
		csrf   string
	}
	login := func(old *http.Cookie) loginResult {
		body, _ := json.Marshal(map[string]string{"password": password})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Origin", "https://geo.test")
		if old != nil {
			req.AddCookie(old)
		}
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("login status=%d", rr.Code)
		}
		var data struct {
			CSRF string `json:"csrfToken"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return loginResult{cookie: rr.Result().Cookies()[0], csrf: data.CSRF}
	}
	first := login(nil)
	second := login(first.cookie)
	if first.cookie.Value == second.cookie.Value {
		t.Fatal("login reused a session token")
	}
	var sessions int
	if err := db.QueryRow(`SELECT count(*) FROM web_sessions`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("sessions after rotation=%d err=%v", sessions, err)
	}
	if _, err := db.Exec(`UPDATE web_sessions SET absolute_expires_at=?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	expired := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	expired.AddCookie(second.cookie)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, expired)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("absolute-expired session status=%d", rr.Code)
	}
	third := login(nil)
	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(third.cookie)
	logout.Header.Set("Origin", "https://geo.test")
	logout.Header.Set("X-CSRF-Token", third.csrf)
	out := httptest.NewRecorder()
	wrapped.ServeHTTP(out, logout)
	if out.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d %s", out.Code, out.Body.String())
	}
	check := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	check.AddCookie(third.cookie)
	denied := httptest.NewRecorder()
	wrapped.ServeHTTP(denied, check)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session status=%d", denied.Code)
	}
}

func TestAccountBackoffPersistsAfterFailedLogin(t *testing.T) {
	svc, db := testService(t)
	ctx := context.Background()
	if err := svc.Provision(ctx, "amber river cedar meadow lantern harbor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, "192.0.2.41", "incorrect passphrase value"); err == nil {
		t.Fatal("invalid password accepted")
	}
	var failures int
	var next sql.NullString
	if err := db.QueryRow(`SELECT consecutive_failures,next_login_at FROM admin_users WHERE id=1`).Scan(&failures, &next); err != nil || failures != 1 || !next.Valid {
		t.Fatalf("backoff failures=%d next=%v err=%v", failures, next, err)
	}
}
