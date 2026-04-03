package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a simple handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestBearerAuth_disabled(t *testing.T) {
	handler := BearerAuth("")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when auth disabled, got %d", rr.Code)
	}
}

func TestBearerAuth_validToken(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", rr.Code)
	}
}

func TestBearerAuth_invalidToken(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid token, got %d", rr.Code)
	}
}

func TestBearerAuth_missingHeader(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with missing header, got %d", rr.Code)
	}
}

func TestBearerAuth_wrongScheme(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	req.Header.Set("Authorization", "Basic secret-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong scheme, got %d", rr.Code)
	}
}

func TestBearerAuth_healthExempt(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health without token, got %d", rr.Code)
	}
}

func TestBearerAuth_readyExempt(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /ready without token, got %d", rr.Code)
	}
}

func TestBearerAuth_metricsExempt(t *testing.T) {
	handler := BearerAuth("secret-key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics without token, got %d", rr.Code)
	}
}
