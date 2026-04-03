package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	payload := map[string]string{"hello": "world"}
	WriteJSON(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/json; charset=utf-8", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("expected %q, got %q", "world", body["hello"])
	}
}

func TestWriteJSON_CustomStatus(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, http.StatusCreated, map[string]int{"count": 42})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusBadRequest, "invalid input")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/json; charset=utf-8", ct)
	}

	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Message != "invalid input" {
		t.Errorf("expected message %q, got %q", "invalid input", body.Error.Message)
	}
	if body.Error.Type != "invalid_request_error" {
		t.Errorf("expected type %q, got %q", "invalid_request_error", body.Error.Type)
	}
}

func TestWriteError_ServerError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusInternalServerError, "something broke")

	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Type != "server_error" {
		t.Errorf("expected type %q, got %q", "server_error", body.Error.Type)
	}
}

func TestWriteBytes(t *testing.T) {
	rec := httptest.NewRecorder()

	data := []byte("raw binary content")
	WriteBytes(rec, http.StatusOK, "application/octet-stream", data)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("expected Content-Type %q, got %q", "application/octet-stream", ct)
	}

	if rec.Body.String() != "raw binary content" {
		t.Errorf("expected body %q, got %q", "raw binary content", rec.Body.String())
	}
}

func TestWriteBytes_TextPlain(t *testing.T) {
	rec := httptest.NewRecorder()

	data := []byte("Hello, world!")
	WriteBytes(rec, http.StatusOK, "text/plain; charset=utf-8", data)

	ct := rec.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "text/plain; charset=utf-8", ct)
	}
}

func TestWriteErrorWithCode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorWithCode(w, http.StatusTooManyRequests, "rate_limited", "slow down")

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != "rate_limited" {
		t.Fatalf("expected code rate_limited, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "slow down" {
		t.Fatalf("expected message 'slow down', got %s", resp.Error.Message)
	}
}

func TestWriteErrorWithRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(r.Context(), requestIDKey{}, "test-req-123")
	r = r.WithContext(ctx)

	WriteErrorWithRequest(w, r, http.StatusBadRequest, "invalid_input", "bad request")

	var resp struct {
		Error struct {
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
			Code      string `json:"code"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.RequestID != "test-req-123" {
		t.Fatalf("expected request_id test-req-123, got %s", resp.Error.RequestID)
	}
	if resp.Error.Code != "invalid_input" {
		t.Fatalf("expected code invalid_input, got %s", resp.Error.Code)
	}
}

func TestHttpStatusToErrorType(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{http.StatusBadRequest, "invalid_request_error"},
		{http.StatusUnauthorized, "authentication_error"},
		{http.StatusForbidden, "permission_error"},
		{http.StatusNotFound, "not_found_error"},
		{http.StatusRequestEntityTooLarge, "invalid_request_error"},
		{http.StatusTooManyRequests, "rate_limit_error"},
		{http.StatusInternalServerError, "server_error"},
		{http.StatusBadGateway, "server_error"},
		{http.StatusServiceUnavailable, "server_error"},
		{http.StatusTeapot, "api_error"}, // 418 - unmapped status
		{http.StatusOK, "api_error"},     // 200 - not typically an error
	}

	for _, tc := range tests {
		got := httpStatusToErrorType(tc.status)
		if got != tc.expected {
			t.Errorf("httpStatusToErrorType(%d): expected %q, got %q", tc.status, tc.expected, got)
		}
	}
}
