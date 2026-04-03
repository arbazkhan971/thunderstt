package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newMultipartRequest builds a multipart POST request with the given form
// fields and a dummy file. It returns the request ready for use in tests.
func newMultipartRequest(t *testing.T, fields map[string]string, fileName string, fileContent []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the file field.
	if fileName != "" {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if fileContent == nil {
			fileContent = []byte("fake audio data")
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("failed to write file content: %v", err)
		}
	}

	// Add other form fields.
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("failed to write field %q: %v", k, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Parse the multipart form so ParseTranscribeRequest can access it.
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	return req
}

func TestParseTranscribeRequest_DefaultModel(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{}, "audio.wav", nil)

	tr, err := ParseTranscribeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tr.File.Close()

	if tr.Model != "auto" {
		t.Errorf("expected model %q, got %q", "auto", tr.Model)
	}
}

func TestParseTranscribeRequest_SpecificModel(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{
		"model": "whisper-tiny",
	}, "audio.wav", nil)

	tr, err := ParseTranscribeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tr.File.Close()

	if tr.Model != "whisper-tiny" {
		t.Errorf("expected model %q, got %q", "whisper-tiny", tr.Model)
	}
}

func TestParseTranscribeRequest_UnknownModel(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{
		"model": "does-not-exist",
	}, "audio.wav", nil)

	_, err := ParseTranscribeRequest(req)
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
}

func TestParseTranscribeRequest_DefaultFormat(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{}, "audio.wav", nil)

	tr, err := ParseTranscribeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tr.File.Close()

	if tr.ResponseFormat != "json" {
		t.Errorf("expected format %q, got %q", "json", tr.ResponseFormat)
	}
}

func TestParseTranscribeRequest_ValidFormats(t *testing.T) {
	formats := []string{"json", "verbose_json", "text", "srt", "vtt"}

	for _, f := range formats {
		req := newMultipartRequest(t, map[string]string{
			"response_format": f,
		}, "audio.wav", nil)

		tr, err := ParseTranscribeRequest(req)
		if err != nil {
			t.Fatalf("format %q: unexpected error: %v", f, err)
		}
		tr.File.Close()

		if tr.ResponseFormat != f {
			t.Errorf("expected format %q, got %q", f, tr.ResponseFormat)
		}
	}
}

func TestParseTranscribeRequest_InvalidFormat(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{
		"response_format": "xml",
	}, "audio.wav", nil)

	_, err := ParseTranscribeRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

func TestParseTranscribeRequest_Language(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{
		"language": "en",
	}, "audio.wav", nil)

	tr, err := ParseTranscribeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tr.File.Close()

	if tr.Language != "en" {
		t.Errorf("expected language %q, got %q", "en", tr.Language)
	}
}

func TestParseTranscribeRequest_MissingFile(t *testing.T) {
	// Build a multipart request with no file field.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("model", "auto")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(32 << 20)

	_, err := ParseTranscribeRequest(req)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseTranscribeRequest_TimestampGranularities(t *testing.T) {
	// Build a request with timestamp_granularities[] = "word"
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("fake audio"))
	writer.WriteField("timestamp_granularities[]", "word")
	writer.WriteField("timestamp_granularities[]", "segment")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(32 << 20)

	tr, err := ParseTranscribeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer tr.File.Close()

	if len(tr.TimestampGranularities) != 2 {
		t.Fatalf("expected 2 granularities, got %d", len(tr.TimestampGranularities))
	}
}

func TestParseTranscribeRequest_InvalidTimestampGranularity(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("fake audio"))
	writer.WriteField("timestamp_granularities[]", "invalid_value")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(32 << 20)

	_, err = ParseTranscribeRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid timestamp granularity, got nil")
	}
}

func TestWantsWordTimestamps_True(t *testing.T) {
	req := &TranscribeRequest{
		TimestampGranularities: []string{"word", "segment"},
	}
	if !req.WantsWordTimestamps() {
		t.Error("expected WantsWordTimestamps to return true")
	}
}

func TestWantsWordTimestamps_False(t *testing.T) {
	req := &TranscribeRequest{
		TimestampGranularities: []string{"segment"},
	}
	if req.WantsWordTimestamps() {
		t.Error("expected WantsWordTimestamps to return false")
	}
}

func TestWantsWordTimestamps_Empty(t *testing.T) {
	req := &TranscribeRequest{
		TimestampGranularities: nil,
	}
	if req.WantsWordTimestamps() {
		t.Error("expected WantsWordTimestamps to return false for nil granularities")
	}
}
