package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

func TestHandleVersion(t *testing.T) {
	SetVersionInfo("1.0.0", "abc123", "2026-01-01")

	eng := engine.NewNoopEngine("test")
	p := pipeline.New(eng)
	srv := NewServer(p, testConfig())

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp versionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", resp.Version)
	}
	if resp.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %s", resp.Commit)
	}
	if resp.GoVersion == "" {
		t.Fatal("go_version should not be empty")
	}
}
