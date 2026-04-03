package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/arbaz/thunderstt/internal/api"
	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

func newTestServer(t *testing.T) *httptest.Server {
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
	return ts
}

func TestIntegration_Health(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestIntegration_Ready(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ready" {
		t.Fatalf("expected ready, got %v", body["status"])
	}
}

func TestIntegration_ListModels(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["object"] != "list" {
		t.Fatalf("expected list object, got %v", body["object"])
	}
}

func TestIntegration_Transcribe_NoFile(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_Transcribe_WithWAV(t *testing.T) {
	ts := newTestServer(t)

	// Create a minimal valid WAV file (44 bytes header + 100 samples).
	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

	// Build multipart form.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "test.wav")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(part, f)
	f.Close()
	writer.WriteField("model", "auto")
	writer.WriteField("response_format", "json")
	writer.Close()

	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// NoopEngine returns empty result, but it should still return 200 with valid JSON.
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestIntegration_Transcribe_InvalidModel(t *testing.T) {
	ts := newTestServer(t)

	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "test.wav")
	f, _ := os.Open(wavPath)
	io.Copy(part, f)
	f.Close()
	writer.WriteField("model", "nonexistent-model-xyz")
	writer.Close()

	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_Metrics(t *testing.T) {
	ts := newTestServer(t)
	// Hit health first to generate some metrics.
	http.Get(ts.URL + "/health")

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	// Verify some prometheus metrics are present.
	if !bytes.Contains([]byte(bodyStr), []byte("thunderstt_http_requests_total")) {
		t.Fatal("expected thunderstt_http_requests_total in metrics output")
	}
}

func TestIntegration_CORS_Preflight(t *testing.T) {
	ts := newTestServer(t)
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/v1/audio/transcriptions", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

// createMinimalWAV writes a valid WAV file with silence.
func createMinimalWAV(t *testing.T, path string) {
	t.Helper()
	// 16-bit mono 16kHz WAV, 100 samples of silence.
	sampleRate := uint32(16000)
	numSamples := 100
	dataSize := uint32(numSamples * 2) // 16-bit = 2 bytes per sample
	fileSize := 36 + dataSize

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// RIFF header
	write := func(b []byte) { f.Write(b) }
	writeU32LE := func(v uint32) {
		b := make([]byte, 4)
		b[0] = byte(v)
		b[1] = byte(v >> 8)
		b[2] = byte(v >> 16)
		b[3] = byte(v >> 24)
		f.Write(b)
	}
	writeU16LE := func(v uint16) {
		b := make([]byte, 2)
		b[0] = byte(v)
		b[1] = byte(v >> 8)
		f.Write(b)
	}

	write([]byte("RIFF"))
	writeU32LE(fileSize)
	write([]byte("WAVE"))
	write([]byte("fmt "))
	writeU32LE(16)             // fmt chunk size
	writeU16LE(1)              // PCM
	writeU16LE(1)              // mono
	writeU32LE(sampleRate)
	writeU32LE(sampleRate * 2) // byte rate
	writeU16LE(2)              // block align
	writeU16LE(16)             // bits per sample
	write([]byte("data"))
	writeU32LE(dataSize)
	// Write silence (zeros).
	silence := make([]byte, dataSize)
	f.Write(silence)
}
