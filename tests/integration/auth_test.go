package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arbaz/thunderstt/internal/api"
	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

func newAuthTestServer(t *testing.T, apiKey string) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      0,
		Model:     "test-noop",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
		APIKey:    apiKey,
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })
	srv := api.NewServer(p, cfg)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func TestIntegration_Auth_NoKeyConfigured(t *testing.T) {
	ts := newAuthTestServer(t, "")
	// All endpoints should work without auth.
	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 without auth, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_Auth_KeyRequired_NoHeader(t *testing.T) {
	ts := newAuthTestServer(t, "test-secret")
	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_Auth_KeyRequired_WrongKey(t *testing.T) {
	ts := newAuthTestServer(t, "test-secret")
	req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_Auth_KeyRequired_CorrectKey(t *testing.T) {
	ts := newAuthTestServer(t, "test-secret")
	req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 with correct key, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_Auth_HealthExempt(t *testing.T) {
	ts := newAuthTestServer(t, "test-secret")
	// Health should work without auth even when key is configured.
	endpoints := []string{"/health", "/ready", "/metrics"}
	for _, ep := range endpoints {
		resp, err := http.Get(ts.URL + ep)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode == 401 {
			t.Fatalf("%s should be exempt from auth, got 401", ep)
		}
		resp.Body.Close()
	}
}

func TestIntegration_Auth_TranscribeRequiresAuth(t *testing.T) {
	ts := newAuthTestServer(t, "test-secret")
	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
