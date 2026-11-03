package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	AdminID         = "1"
	IdleTimeout     = 30 * time.Minute
	AbsoluteTimeout = 12 * time.Hour
	passwordMemory  = 19 * 1024
	passwordTime    = 2
	passwordThreads = 1
)

var ErrRateLimited = errors.New("authentication failed")

type Principal struct{ ID, Label string }
type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// AuditTx inserts an actor-attributed event in the caller's business transaction.
// It is a no-op for internal operations that do not carry an authenticated principal.
func AuditTx(ctx context.Context, tx *sql.Tx, action, resourceType, resourceID, requestID string, details any) error {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return nil
	}
	if len(p.ID) > 64 || len(p.Label) > 64 || len(action) > 64 || len(resourceType) > 64 || len(resourceID) > 128 || len(requestID) > 128 {
		return errors.New("audit metadata exceeds limit")
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if len(b) > 2048 {
		return errors.New("audit details exceed limit")
	}
	id, err := randomHex(16)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_events(id,occurred_at,actor_id,actor_label,action,resource_type,resource_id,request_id,payload_version,details) VALUES(?,?,?,?,?,?,?,?,1,?)`, id, time.Now().UTC().Format(time.RFC3339Nano), p.ID, p.Label, action, resourceType, resourceID, requestID, string(b))
	return err
}

type Config struct {
	PublicOrigin      string
	CookieSecure      bool
	CookieName        string
	TrustedProxyCIDRs []string
}
type Service struct {
	db              *sql.DB
	cfg             Config
	dummy           string
	limiter         *loginLimiter
	mutationLimiter *loginLimiter
	kdf             chan struct{}
}

func New(db *sql.DB, cfg Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("auth database is required")
	}
	u, err := validateOrigin(cfg.PublicOrigin)
	if err != nil {
		return nil, err
	}
	cfg.PublicOrigin = u
	if cfg.CookieName == "" {
		if cfg.CookieSecure {
			cfg.CookieName = "__Host-geoentity_session"
		} else {
			cfg.CookieName = "geoentity_session"
		}
	}
	if cfg.CookieSecure && cfg.CookieName != "__Host-geoentity_session" {
		return nil, errors.New("secure production cookie must use __Host- prefix")
	}
	if !cfg.CookieSecure && cfg.CookieName == "__Host-geoentity_session" {
		return nil, errors.New("__Host- cookie requires Secure")
	}
	for _, cidr := range cfg.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR")
		}
	}
	dummy, err := hashPassword("V2.6 dummy verification passphrase")
	if err != nil {
		return nil, err
	}
	if cfg.CookieSecure && !strings.HasPrefix(cfg.PublicOrigin, "https://") {
		return nil, errors.New("secure cookies require an HTTPS public origin")
	}
	return &Service{db: db, cfg: cfg, dummy: dummy, limiter: newLoginLimiter(4096), mutationLimiter: newLoginLimiter(4096), kdf: make(chan struct{}, 1)}, nil
}

func NormalizePassword(password string) string { return norm.NFC.String(password) }
func ValidatePassword(password string) error {
	if len(password) > 1024 {
		return errors.New("passphrase is too long")
	}
	n := NormalizePassword(password)
	if len(n) > 1024 {
		return errors.New("passphrase is too long")
	}
	if len([]rune(n)) < 15 {
		return errors.New("passphrase must contain at least 15 Unicode code points")
	}
	if isBlocked(n) {
		return errors.New("passphrase is too common or compromised")
	}
	return nil
}
func isBlocked(s string) bool {
	x := strings.ToLower(strings.TrimSpace(s))
	for _, bad := range []string{"passwordpassword", "password123456", "password1234567", "password12345678", "123456789012345", "qwertyuiopasdfg", "letmeinletmein", "adminadminadmin", "administrator1234", "welcomehomewelcome", "correcthorsebatterystaple", "correct horse battery staple", "geoentityadministrator"} {
		if x == bad {
			return true
		}
	}
	return false
}
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(NormalizePassword(password)), salt, passwordTime, passwordMemory, passwordThreads, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordTime, passwordThreads, hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}
func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, rounds, threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &rounds, &threads); err != nil || memory < 19*1024 || rounds < 2 || threads < 1 || memory > 256*1024 || rounds > 10 || threads > 8 {
		return false
	}
	saltHex, keyHex := parts[4], parts[5]
	salt, e1 := hex.DecodeString(saltHex)
	expected, e2 := hex.DecodeString(keyHex)
	if e1 != nil || e2 != nil || len(salt) < 16 || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(NormalizePassword(password)), salt, rounds, memory, uint8(threads), uint32(len(expected)))
	return subtleEqual(actual, expected)
}
func subtleEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func (s *Service) Provision(ctx context.Context, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	verifier, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_users(id,password_verifier,created_at,updated_at) VALUES(1,?,?,?)`, verifier, now, now)
	if err != nil {
		return errors.New("administrator already provisioned or database unavailable")
	}
	if err = AuditTx(WithPrincipal(ctx, Principal{ID: AdminID, Label: "administrator"}), tx, "admin.provisioned", "admin", AdminID, "", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) RotatePassword(ctx context.Context, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	verifier, err := hashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE admin_users SET password_verifier=?,consecutive_failures=0,next_login_at=NULL,updated_at=? WHERE id=1`, verifier, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("administrator has not been provisioned")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM web_sessions`); err != nil {
		return err
	}
	auditID, err := randomHex(16)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_events(id,occurred_at,actor_id,actor_label,action,resource_type,resource_id,payload_version,details) VALUES(?,?,?,?,?,?,?,1,'{}')`, auditID, time.Now().UTC().Format(time.RFC3339Nano), AdminID, "administrator", "password.rotated", "admin", "1"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) HasAdmin(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM admin_users WHERE id=1`).Scan(&n)
	return n == 1, err
}
func (s *Service) Login(ctx context.Context, ip, password string) (string, string, error) {
	if len(password) > 1024 {
		return "", "", errors.New("authentication failed")
	}
	if !s.limiter.allow(ip) {
		return "", "", ErrRateLimited
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return "", "", ErrRateLimited
	}
	var verifier string
	var failures int
	var next sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT password_verifier,consecutive_failures,next_login_at FROM admin_users WHERE id=1`).Scan(&verifier, &failures, &next)
	if err != nil {
		verifier = s.dummy
		failures = 0
	}
	if next.Valid {
		until, e := time.Parse(time.RFC3339Nano, next.String)
		if e == nil && time.Now().Before(until) {
			_ = verifyPassword(password, verifier)
			return "", "", errors.New("authentication failed")
		}
	}
	valid := verifyPassword(password, verifier)
	if err != nil || !valid {
		if err == nil {
			failures++
			delay := time.Duration(1<<min(failures-1, 6)) * time.Second
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
			_, _ = s.db.ExecContext(ctx, `UPDATE admin_users SET consecutive_failures=?,next_login_at=?,updated_at=? WHERE id=1 AND password_verifier=?`, failures, time.Now().Add(delay).UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), verifier)
		}
		return "", "", errors.New("authentication failed")
	}
	secret := make([]byte, 32)
	csrf := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", err
	}
	if _, err = rand.Read(csrf); err != nil {
		return "", "", err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var currentVerifier string
	if err = tx.QueryRowContext(ctx, `SELECT password_verifier FROM admin_users WHERE id=1`).Scan(&currentVerifier); err != nil || currentVerifier != verifier {
		return "", "", errors.New("authentication failed")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE admin_users SET consecutive_failures=0,next_login_at=NULL,updated_at=? WHERE id=1`, now.Format(time.RFC3339Nano)); err != nil {
		return "", "", err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM web_sessions WHERE absolute_expires_at<=? OR last_seen_at<=?`, now.Format(time.RFC3339Nano), now.Add(-IdleTimeout).Format(time.RFC3339Nano))
	if err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(secret)
	_, err = tx.ExecContext(ctx, `INSERT INTO web_sessions(token_hash,csrf_hash,created_at,last_seen_at,absolute_expires_at) VALUES(?,?,?,?,?)`, digest([]byte(token)), digest([]byte(hex.EncodeToString(csrf))), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Add(AbsoluteTimeout).Format(time.RFC3339Nano))
	if err != nil {
		return "", "", err
	}
	if err = AuditTx(WithPrincipal(ctx, Principal{ID: AdminID, Label: "administrator"}), tx, "session.login", "session", "", "", map[string]any{}); err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return token, hex.EncodeToString(csrf), nil
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/auth/login" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.login(w, r)
			return
		}
		if r.URL.Path == "/api/auth/session" {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.sessionInfo(w, r)
			return
		}
		if r.URL.Path == "/api/auth/logout" && r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		token := ""
		if c, e := r.Cookie(s.cfg.CookieName); e == nil {
			token = c.Value
		}
		if len(token) != 64 {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		csrf := r.Header.Get("X-CSRF-Token")
		var csrfHash, created, last, absolute string
		err := s.db.QueryRowContext(r.Context(), `SELECT csrf_hash,created_at,last_seen_at,absolute_expires_at FROM web_sessions WHERE token_hash=?`, digest([]byte(token))).Scan(&csrfHash, &created, &last, &absolute)
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		now := time.Now().UTC()
		lastAt, e1 := time.Parse(time.RFC3339Nano, last)
		absoluteAt, e2 := time.Parse(time.RFC3339Nano, absolute)
		if e1 != nil || e2 != nil || now.After(absoluteAt) || now.Sub(lastAt) > IdleTimeout {
			_, _ = s.db.ExecContext(r.Context(), `DELETE FROM web_sessions WHERE token_hash=?`, digest([]byte(token)))
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if unsafe {
			if !s.originAllowed(r) || len(csrf) != 64 || digest([]byte(csrf)) != csrfHash {
				http.Error(w, "request rejected", http.StatusForbidden)
				return
			}
		}
		if unsafe || strings.HasPrefix(r.URL.Path, "/api/imports/") || strings.HasPrefix(r.URL.Path, "/api/exports/") {
			perIP, global := 60, 600
			if strings.HasPrefix(r.URL.Path, "/api/imports/") || strings.HasPrefix(r.URL.Path, "/api/exports/") {
				perIP, global = 5, 50
			}
			if !s.mutationLimiter.allowWith(s.clientIP(r), perIP, global) {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
		}
		if r.URL.Path == "/api/auth/logout" {
			tx, e := s.db.BeginTx(r.Context(), nil)
			if e != nil {
				http.Error(w, "request failed", http.StatusInternalServerError)
				return
			}
			_, e = tx.ExecContext(r.Context(), `DELETE FROM web_sessions WHERE token_hash=?`, digest([]byte(token)))
			if e == nil {
				e = AuditTx(WithPrincipal(r.Context(), Principal{ID: AdminID, Label: "administrator"}), tx, "session.logout", "session", "", RequestID(r.Context()), map[string]any{})
			}
			if e == nil {
				e = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if e != nil {
				http.Error(w, "request failed", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: s.cfg.CookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if now.Sub(lastAt) > time.Minute {
			_, _ = s.db.ExecContext(r.Context(), `UPDATE web_sessions SET last_seen_at=? WHERE token_hash=?`, now.Format(time.RFC3339Nano), digest([]byte(token)))
		}
		ctx := WithPrincipal(r.Context(), Principal{ID: AdminID, Label: "administrator"})
		reqID := r.Header.Get("X-Request-ID")
		if len(reqID) > 128 {
			reqID = ""
		}
		if reqID != "" {
			ctx = context.WithValue(ctx, requestIDKey{}, reqID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type requestIDKey struct{}

func RequestID(ctx context.Context) string { v, _ := ctx.Value(requestIDKey{}).(string); return v }
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	if !s.originAllowed(r) {
		http.Error(w, "request rejected", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	ip := s.clientIP(r)
	token, csrf, err := s.Login(r.Context(), ip, input.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrRateLimited) {
			status = http.StatusTooManyRequests
		}
		http.Error(w, "authentication failed", status)
		return
	}
	if old, e := r.Cookie(s.cfg.CookieName); e == nil && len(old.Value) == 64 {
		if _, e = s.db.ExecContext(r.Context(), `DELETE FROM web_sessions WHERE token_hash=?`, digest([]byte(old.Value))); e != nil {
			_, _ = s.db.ExecContext(r.Context(), `DELETE FROM web_sessions WHERE token_hash=?`, digest([]byte(token)))
			http.Error(w, "authentication failed", http.StatusInternalServerError)
			return
		}
	}
	maxAge := int(AbsoluteTimeout.Seconds())
	http.SetCookie(w, &http.Cookie{Name: s.cfg.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "csrfToken": csrf, "expiresInSeconds": maxAge})
}
func (s *Service) sessionInfo(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie(s.cfg.CookieName); e == nil && len(c.Value) == 64 {
		if !s.originAllowed(r) {
			http.Error(w, "request rejected", http.StatusForbidden)
			return
		}
		var hash string
		var last, absolute string
		if s.db.QueryRowContext(r.Context(), `SELECT csrf_hash,last_seen_at,absolute_expires_at FROM web_sessions WHERE token_hash=?`, digest([]byte(c.Value))).Scan(&hash, &last, &absolute) == nil {
			la, e1 := time.Parse(time.RFC3339Nano, last)
			aa, e2 := time.Parse(time.RFC3339Nano, absolute)
			now := time.Now().UTC()
			if e1 == nil && e2 == nil && now.Sub(la) <= IdleTimeout && now.Before(aa) {
				csrf := make([]byte, 32)
				if _, e := rand.Read(csrf); e == nil {
					csrfToken := hex.EncodeToString(csrf)
					if _, err := s.db.ExecContext(r.Context(), `UPDATE web_sessions SET csrf_hash=? WHERE token_hash=?`, digest([]byte(csrfToken)), digest([]byte(c.Value))); err != nil {
						http.Error(w, "session unavailable", http.StatusInternalServerError)
						return
					}
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "csrfToken": csrfToken})
					return
				}
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
}
func (s *Service) originAllowed(r *http.Request) bool {
	value := r.Header.Get("Origin")
	if value == "" {
		value = r.Header.Get("Referer")
		if value == "" {
			return false
		}
		u, err := url.Parse(value)
		if err != nil || u.User != nil || u.Host == "" {
			return false
		}
		value = u.Scheme + "://" + u.Host
	}
	parsed, err := validateOrigin(value)
	return err == nil && parsed == s.cfg.PublicOrigin
}
func validateOrigin(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u == nil || u.User != nil || u.Host == "" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("invalid public origin")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}

type loginLimiter struct {
	mu  sync.Mutex
	m   map[string]bucket
	cap int
}
type bucket struct {
	start   time.Time
	count   int
	touched time.Time
}

func newLoginLimiter(capacity int) *loginLimiter {
	return &loginLimiter{m: make(map[string]bucket), cap: capacity}
}
func (l *loginLimiter) allow(ip string) bool {
	return l.allowWith(ip, 10, 120)
}
func (l *loginLimiter) allowWith(ip string, perIP, globalLimit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, v := range l.m {
		if now.Sub(v.touched) > 10*time.Minute {
			delete(l.m, k)
		}
	}
	if len(l.m) >= l.cap {
		if _, ok := l.m[ip]; !ok {
			return false
		}
	}
	b := l.m[ip]
	if now.Sub(b.start) >= time.Minute {
		b = bucket{start: now}
	}
	b.count++
	b.touched = now
	l.m[ip] = b
	global := l.m["*"]
	if now.Sub(global.start) >= time.Minute {
		global = bucket{start: now}
	}
	global.count++
	global.touched = now
	l.m["*"] = global
	return b.count <= perIP && global.count <= globalLimit
}
func (s *Service) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	trusted := false
	for _, raw := range s.cfg.TrustedProxyCIDRs {
		_, cidr, _ := net.ParseCIDR(raw)
		if cidr.Contains(peer) {
			trusted = true
			break
		}
	}
	if trusted {
		if candidate := r.Header.Get("X-Real-IP"); candidate != "" {
			if ip := net.ParseIP(candidate); ip != nil {
				return ip.String()
			}
		}
	}
	if peer != nil {
		return peer.String()
	}
	return "unknown"
}
