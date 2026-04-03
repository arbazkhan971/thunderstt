package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

func testConfig() *config.Config {
	return &config.Config{
		Host:      "0.0.0.0",
		Port:      8080,
		Model:     "test",
		Workers:   1,
		LogLevel:  "info",
		ModelsDir: "/tmp",
	}
}

func TestHandleHealth(t *testing.T) {
	eng := engine.NewNoopEngine("test-model")
	p := pipeline.New(eng)
	srv := NewServer(p, testConfig())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", body.Status)
	}
	if body.Uptime == "" {
		t.Error("expected uptime field to be present")
	}
}

func TestHandleReady_NotReady(t *testing.T) {
	// Create a pipeline with a nil engine so it is NOT ready.
	p := pipeline.New(nil)
	srv := NewServer(p, testConfig())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body readyResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "not_ready" {
		t.Errorf("expected status %q, got %q", "not_ready", body.Status)
	}
	if body.Model != "" {
		t.Errorf("expected empty model, got %q", body.Model)
	}
	if body.SystemInfo != nil {
		t.Error("expected system info to be nil for not-ready response")
	}
}

func TestHandleReady_Ready(t *testing.T) {
	eng := engine.NewNoopEngine("whisper-tiny")
	p := pipeline.New(eng)
	srv := NewServer(p, testConfig())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body readyResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "ready" {
		t.Errorf("expected status %q, got %q", "ready", body.Status)
	}
	if body.Model != "whisper-tiny" {
		t.Errorf("expected model %q, got %q", "whisper-tiny", body.Model)
	}
	if body.QueueMax != 1 {
		t.Errorf("expected queue_capacity 1, got %d", body.QueueMax)
	}
	if body.SystemInfo == nil {
		t.Fatal("expected system info to be present")
	}
	if body.SystemInfo.GoVersion == "" {
		t.Error("expected go_version to be set")
	}
	if body.SystemInfo.NumCPU < 1 {
		t.Errorf("expected num_cpu >= 1, got %d", body.SystemInfo.NumCPU)
	}
	if body.SystemInfo.NumGoroutine < 1 {
		t.Errorf("expected num_goroutine >= 1, got %d", body.SystemInfo.NumGoroutine)
	}
}
