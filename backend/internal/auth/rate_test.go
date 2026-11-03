package auth

import (
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestLoginLimiterBoundsStateAndFailsClosedAtCapacity(t *testing.T) {
	l := newLoginLimiter(8)
	for i := 0; i < 7; i++ {
		if !l.allowWith("192.0.2."+strconv.Itoa(i+1), 100, 1000) {
			t.Fatal("unexpected bucket rejection")
		}
	}
	if len(l.m) > 8 {
		t.Fatalf("limiter state exceeded cap: %d", len(l.m))
	}
	if l.allowWith("198.51.100.22", 100, 1000) {
		t.Fatal("new source should fail closed at capacity")
	}
	for i := 0; i < 15; i++ {
		_ = l.allowWith("192.0.2.1", 100, 1000)
	}
	if len(l.m) > 8 {
		t.Fatalf("repeated attempts grew limiter map to %d", len(l.m))
	}
}

func TestClientIPIgnoresUntrustedForwardingHeaders(t *testing.T) {
	s := &Service{cfg: Config{TrustedProxyCIDRs: []string{"10.0.0.0/8"}}}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.8:4567"
	r.Header.Set("X-Real-IP", "203.0.113.9")
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	if got := s.clientIP(r); got != "192.0.2.8" {
		t.Fatalf("untrusted forwarding header selected %q", got)
	}
	r.RemoteAddr = "10.0.0.2:4567"
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("trusted proxy client IP=%q", got)
	}
}
