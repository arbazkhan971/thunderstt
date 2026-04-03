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

func TestHandleListModels(t *testing.T) {
	eng := engine.NewNoopEngine("test-model")
	p := pipeline.New(eng)
	cfg := &config.Config{
		Host:      "0.0.0.0",
		Port:      8080,
		Model:     "test",
		Workers:   1,
		LogLevel:  "info",
		ModelsDir: "/tmp",
	}
	srv := NewServer(p, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/json; charset=utf-8", ct)
	}

	var body modelListResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Object != "list" {
		t.Errorf("expected object %q, got %q", "list", body.Object)
	}

	if len(body.Data) != len(availableModels) {
		t.Fatalf("expected %d models, got %d", len(availableModels), len(body.Data))
	}

	// Verify every model has the required fields.
	for i, m := range body.Data {
		if m.ID == "" {
			t.Errorf("model[%d]: ID is empty", i)
		}
		if m.Object != "model" {
			t.Errorf("model[%d]: expected object %q, got %q", i, "model", m.Object)
		}
		if m.OwnedBy == "" {
			t.Errorf("model[%d]: OwnedBy is empty", i)
		}
	}

	// Verify known models are present.
	ids := make(map[string]bool)
	for _, m := range body.Data {
		ids[m.ID] = true
	}
	expectedIDs := []string{"whisper-large-v3-turbo", "whisper-tiny", "parakeet-tdt-0.6b-v3"}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("expected model %q to be in the list", id)
		}
	}
}
