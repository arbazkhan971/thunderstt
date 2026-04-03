package integration

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/arbaz/thunderstt/internal/api"
	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
	sdk "github.com/arbaz/thunderstt/pkg/thunderstt"
)

func newSDKTestServer(t *testing.T) (*httptest.Server, *sdk.Client) {
	t.Helper()
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      0,
		Model:     "test-noop",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })

	srv := api.NewServer(p, cfg)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	client := sdk.NewClient(ts.URL)
	return ts, client
}

func TestSDK_Health(t *testing.T) {
	_, client := newSDKTestServer(t)
	resp, err := client.Health()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected ok, got %s", resp.Status)
	}
}

func TestSDK_ListModels(t *testing.T) {
	_, client := newSDKTestServer(t)
	resp, err := client.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object != "list" {
		t.Fatalf("expected list, got %s", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected models in list")
	}
}

func TestSDK_Transcribe_FilePath(t *testing.T) {
	_, client := newSDKTestServer(t)

	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

	resp, err := client.Transcribe(sdk.TranscribeRequest{
		FilePath:       wavPath,
		Model:          "auto",
		ResponseFormat: "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	// NoopEngine returns empty text, but response should be valid.
	_ = resp.Text
}

func TestSDK_Transcribe_Reader(t *testing.T) {
	_, client := newSDKTestServer(t)

	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

	f, err := os.Open(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	resp, err := client.Transcribe(sdk.TranscribeRequest{
		Reader:         f,
		Filename:       "test.wav",
		Model:          "auto",
		ResponseFormat: "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Text
}

func TestSDK_Transcribe_WithOptions(t *testing.T) {
	_, client := newSDKTestServer(t)

	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

	resp, err := client.Transcribe(sdk.TranscribeRequest{
		FilePath:               wavPath,
		Model:                  "auto",
		Language:               "en",
		ResponseFormat:         "verbose_json",
		TimestampGranularities: []string{"word", "segment"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Text
}

func TestSDK_WithAPIKey(t *testing.T) {
	// Create server with API key.
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      0,
		Model:     "test-noop",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
		APIKey:    "test-secret-key",
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })

	srv := api.NewServer(p, cfg)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	// Client without key should fail on /v1 endpoints.
	noKeyClient := sdk.NewClient(ts.URL)
	_, err := noKeyClient.ListModels()
	if err == nil {
		t.Fatal("expected error without API key")
	}

	// Health should work without key.
	resp, err := noKeyClient.Health()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatal("health should work without auth")
	}

	// Client with key should succeed.
	authClient := sdk.NewClient(ts.URL, sdk.WithAPIKey("test-secret-key"))
	models, err := authClient.ListModels()
	if err != nil {
		t.Fatalf("expected success with key: %v", err)
	}
	if len(models.Data) == 0 {
		t.Fatal("expected models")
	}
}
