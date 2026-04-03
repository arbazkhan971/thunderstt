package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimiter_allow(t *testing.T) {
	// Burst of 3: first 3 requests should be allowed.
	rl := NewRateLimiter(1, 3)

	for i := 0; i < 3; i++ {
		if !rl.allow("10.0.0.1") {
			t.Fatalf("request %d should have been allowed (burst=3)", i+1)
		}
	}
}

func TestRateLimiter_exceedsBurst(t *testing.T) {
	// Burst of 2: third request should be denied.
	rl := NewRateLimiter(1, 2)

	if !rl.allow("10.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if !rl.allow("10.0.0.1") {
		t.Fatal("second request should be allowed")
	}
	if rl.allow("10.0.0.1") {
		t.Fatal("third request should be denied (burst=2)")
	}
}

func TestRateLimit_middleware(t *testing.T) {
	// Burst of 1: only one request allowed, second gets 429.
	middleware := RateLimit(1, 1)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on first request, got %d", rec.Code)
	}

	// Second request should be rate limited.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on second request, got %d", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter != "1" {
		t.Errorf("expected Retry-After header %q, got %q", "1", retryAfter)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "rate limit exceeded") {
		t.Errorf("expected body to contain %q, got %q", "rate limit exceeded", body)
	}
}

func TestRateLimit_differentIPs(t *testing.T) {
	// Burst of 1: each IP gets its own bucket.
	middleware := RateLimit(1, 1)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP uses its token.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1111"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for first IP, got %d", rec.Code)
	}

	// Second IP should still be allowed (separate bucket).
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:2222"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for second IP, got %d", rec.Code)
	}

	// First IP again should be denied.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:3333"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for first IP second request, got %d", rec.Code)
	}
}
