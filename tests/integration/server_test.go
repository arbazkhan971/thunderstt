package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// newTestServerWithRateLimit creates a test server with a custom rate limit.
func newTestServerWithRateLimit(t *testing.T, rate float64, burst int) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      0,
		Model:     "test-noop",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
		RateLimit: rate,
		RateBurst: burst,
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })

	srv := api.NewServer(p, cfg)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

// buildTranscribeRequest creates a multipart body for the transcribe endpoint
// with the given form fields. The "file" field is always populated with a
// minimal WAV file.
func buildTranscribeRequest(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

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
	for k, v := range fields {
		writer.WriteField(k, v)
	}
	writer.Close()
	return &body, writer.FormDataContentType()
}

func TestIntegration_Transcribe_AllFormats(t *testing.T) {
	ts := newTestServer(t)

	formats := []struct {
		name        string
		contentType string
	}{
		{"json", "application/json"},
		{"verbose_json", "application/json"},
		{"text", "text/plain"},
		{"srt", "text/plain"},
		{"vtt", "text/plain"},
	}

	for _, fmt := range formats {
		t.Run(fmt.name, func(t *testing.T) {
			wavPath := filepath.Join(t.TempDir(), "test.wav")
			createMinimalWAV(t, wavPath)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, _ := writer.CreateFormFile("file", "test.wav")
			f, _ := os.Open(wavPath)
			io.Copy(part, f)
			f.Close()
			writer.WriteField("model", "auto")
			writer.WriteField("response_format", fmt.name)
			writer.Close()

			resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", writer.FormDataContentType(), &body)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				t.Fatalf("format %s: expected 200, got %d: %s", fmt.name, resp.StatusCode, string(bodyBytes))
			}

			// Verify the Content-Type header matches expectations.
			ct := resp.Header.Get("Content-Type")
			if !bytes.Contains([]byte(ct), []byte(fmt.contentType)) {
				t.Errorf("format %s: expected Content-Type containing %q, got %q", fmt.name, fmt.contentType, ct)
			}
		})
	}
}

func TestIntegration_Transcribe_WithLanguage(t *testing.T) {
	ts := newTestServer(t)

	body, contentType := buildTranscribeRequest(t, map[string]string{
		"model":           "auto",
		"language":        "en",
		"response_format": "json",
	})

	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	// The response should at least contain a "text" field.
	if _, ok := result["text"]; !ok {
		t.Fatal("expected \"text\" field in JSON response")
	}
}

func TestIntegration_Transcribe_WithWordTimestamps(t *testing.T) {
	ts := newTestServer(t)

	wavPath := filepath.Join(t.TempDir(), "test.wav")
	createMinimalWAV(t, wavPath)

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
	writer.WriteField("response_format", "verbose_json")
	writer.WriteField("timestamp_granularities[]", "word")
	writer.Close()

	resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode and check we got a valid verbose_json response.
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode verbose_json response: %v", err)
	}
	if _, ok := result["text"]; !ok {
		t.Fatal("expected \"text\" field in verbose_json response")
	}
}

func TestIntegration_NotFound(t *testing.T) {
	ts := newTestServer(t)

	paths := []string{
		"/nonexistent",
		"/v1/nonexistent",
		"/v2/audio/transcriptions",
		"/foo/bar/baz",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			// chi returns 404 or 405 for unmatched routes.
			if resp.StatusCode != 404 && resp.StatusCode != 405 {
				t.Fatalf("path %s: expected 404 or 405, got %d", path, resp.StatusCode)
			}
		})
	}
}

func TestIntegration_RateLimit(t *testing.T) {
	// Create a server with a very low rate limit so we can trigger 429 quickly.
	ts := newTestServerWithRateLimit(t, 5, 5)

	var got429 atomic.Bool
	// Send 300 rapid requests to /health.
	for i := 0; i < 300; i++ {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429.Store(true)
			break
		}
	}

	if !got429.Load() {
		t.Fatal("expected at least one 429 Too Many Requests response after 300 rapid requests")
	}
}

func TestIntegration_Concurrent_Transcribe(t *testing.T) {
	ts := newTestServer(t)

	const numRequests = 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			body, contentType := buildTranscribeRequest(t, map[string]string{
				"model":           "auto",
				"response_format": "json",
			})

			resp, err := http.Post(ts.URL+"/v1/audio/transcriptions", contentType, body)
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				errors <- fmt.Errorf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
				return
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent request failed: %v", err)
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
