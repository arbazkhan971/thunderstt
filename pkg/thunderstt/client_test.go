package thunderstt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8080")
	if c.BaseURL != "http://localhost:8080" {
		t.Fatal("wrong base URL")
	}
	if c.APIKey != "" {
		t.Fatal("API key should be empty")
	}
}

func TestNewClient_withOptions(t *testing.T) {
	c := NewClient("http://localhost:8080", WithAPIKey("test-key"))
	if c.APIKey != "test-key" {
		t.Fatal("API key not set")
	}
}

func TestClient_Health(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Uptime: "1m30s"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	resp, err := c.Health()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected ok, got %s", resp.Status)
	}
}

func TestClient_ListModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ModelList{
			Object: "list",
			Data: []Model{
				{ID: "test-model", Object: "model", OwnedBy: "test"},
			},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	resp, err := c.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Data))
	}
}

func TestClient_Transcribe_withFilePath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		r.ParseMultipartForm(10 << 20)
		if r.FormValue("model") != "auto" {
			t.Fatalf("expected model auto, got %s", r.FormValue("model"))
		}
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "hello world"})
	}))
	defer ts.Close()

	// Create a temp file.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake audio data"), 0644)

	c := NewClient(ts.URL)
	resp, err := c.Transcribe(TranscribeRequest{FilePath: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello world" {
		t.Fatalf("expected 'hello world', got %s", resp.Text)
	}
}

func TestClient_Transcribe_withReader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "from reader"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	resp, err := c.Transcribe(TranscribeRequest{
		Reader:   strings.NewReader("fake audio"),
		Filename: "test.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "from reader" {
		t.Fatalf("expected 'from reader', got %s", resp.Text)
	}
}

func TestClient_Transcribe_noFileOrReader(t *testing.T) {
	c := NewClient("http://localhost:9999")
	_, err := c.Transcribe(TranscribeRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Transcribe_withAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "authed"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, WithAPIKey("test-key"))
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake"), 0644)

	resp, err := c.Transcribe(TranscribeRequest{FilePath: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "authed" {
		t.Fatalf("expected 'authed', got %s", resp.Text)
	}
}

func TestClient_Transcribe_serverError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake"), 0644)

	_, err := c.Transcribe(TranscribeRequest{FilePath: tmpFile})
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestClient_Version(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(VersionResponse{
			Version: "1.0.0", GoVersion: "go1.23", OS: "darwin", Arch: "arm64",
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	resp, err := c.Version()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", resp.Version)
	}
}

func TestClient_Translate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/translations" {
			t.Fatalf("expected translations path, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TranscribeResponse{Text: "translated text"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake audio"), 0644)

	resp, err := c.Translate(TranscribeRequest{FilePath: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "translated text" {
		t.Fatalf("expected 'translated text', got %s", resp.Text)
	}
}
