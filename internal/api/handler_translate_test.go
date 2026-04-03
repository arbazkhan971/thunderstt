package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleTranslate_Success(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model": "auto", "response_format": "json",
	}, minimalWAV())
	req.URL.Path = "/v1/audio/translations"
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// The response should be valid JSON.
	var raw json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestHandleTranslate_NoFile(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req := httptest.NewRequest("POST", "/v1/audio/translations", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=foo")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleTranslate_InvalidExtension(t *testing.T) {
	srv := newTestServerForTranscribe(t)
	req, _ := buildMultipartRequest(t, map[string]string{
		"model": "auto", "_filename": "test.exe",
	}, minimalWAV())
	req.URL.Path = "/v1/audio/translations"
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
