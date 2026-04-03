package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

// newTestServerForTranscribe creates a Server backed by a NoopEngine pipeline,
// suitable for handler-level tests that exercise the full middleware stack.
func newTestServerForTranscribe(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      8080,
		Model:     "test",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })
	return NewServer(p, cfg)
}

// buildMultipartRequest constructs a multipart/form-data POST request.
// If wavData is non-nil it is attached as the "file" field. The special key
// "_filename" in fields overrides the default filename "test.wav".
func buildMultipartRequest(t *testing.T, fields map[string]string, wavData []byte) (*http.Request, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if wavData != nil {
		filename := "test.wav"
		if fn, ok := fields["_filename"]; ok {
			filename = fn
			delete(fields, "_filename")
		}
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(wavData)
	}

	for k, v := range fields {
		writer.WriteField(k, v)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

// minimalWAV returns a valid 16-bit mono 16 kHz WAV containing 0.1 s of silence.
func minimalWAV() []byte {
	var buf bytes.Buffer
	sampleRate := uint32(16000)
	numSamples := 1600 // 0.1 seconds
	dataSize := uint32(numSamples * 2)
	fileSize := 36 + dataSize

	writeU32 := func(v uint32) {
		b := [4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
		buf.Write(b[:])
	}
	writeU16 := func(v uint16) {
		b := [2]byte{byte(v), byte(v >> 8)}
		buf.Write(b[:])
	}

	buf.Write([]byte("RIFF"))
	writeU32(fileSize)
	buf.Write([]byte("WAVE"))
	buf.Write([]byte("fmt "))
	writeU32(16)        // chunk size
	writeU16(1)         // PCM
	writeU16(1)         // mono
	writeU32(sampleRate)
	writeU32(sampleRate * 2) // byte rate
	writeU16(2)              // block align
	writeU16(16)             // bits per sample
	buf.Write([]byte("data"))
	writeU32(dataSize)
	buf.Write(make([]byte, dataSize))
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleTranscribe_Success(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "json",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content-type, got %q", ct)
	}
	// The response should be valid JSON.
	var raw json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestHandleTranscribe_VerboseJSON(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "verbose_json",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content-type, got %q", ct)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestHandleTranscribe_TextFormat(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "text",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
}

func TestHandleTranscribe_SRTFormat(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "srt",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
}

func TestHandleTranscribe_VTTFormat(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "vtt",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
	// VTT output should start with "WEBVTT".
	body := rec.Body.String()
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Errorf("expected VTT body to start with WEBVTT, got: %q", body[:min(len(body), 40)])
	}
}

func TestHandleTranscribe_InvalidFileExtension(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":     "auto",
		"_filename": "test.exe",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "unsupported file extension") {
		t.Errorf("expected error about unsupported file extension, got: %s", body)
	}
}

func TestHandleTranscribe_NoFile(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	// Build a multipart request with no file attached (wavData == nil).
	req, _ := buildMultipartRequest(t, map[string]string{
		"model": "auto",
	}, nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "file") {
		t.Errorf("expected error mentioning 'file', got: %s", body)
	}
}

func TestHandleTranscribe_InvalidModel(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model": "nonexistent-model-xyz",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "unknown model") {
		t.Errorf("expected error about unknown model, got: %s", body)
	}
}

func TestHandleTranscribe_InvalidFormat(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model":           "auto",
		"response_format": "csv",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid response_format") {
		t.Errorf("expected error about invalid response_format, got: %s", body)
	}
}

func TestHandleTranscribe_PipelineNotReady(t *testing.T) {
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      8080,
		Model:     "test",
		Workers:   2,
		LogLevel:  "error",
		ModelsDir: t.TempDir(),
	}
	// A pipeline created with a nil engine reports Ready() == false.
	p := pipeline.New(nil)
	t.Cleanup(func() { p.Close() })
	srv := NewServer(p, cfg)

	req, _ := buildMultipartRequest(t, map[string]string{
		"model": "auto",
	}, minimalWAV())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "not ready") || !strings.Contains(body, "model_not_ready") {
		t.Errorf("expected model_not_ready error, got: %s", body)
	}
}

