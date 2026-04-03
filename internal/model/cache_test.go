package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCacheDir(t *testing.T) {
	// Ensure the env var is NOT set so we get the default path.
	t.Setenv("THUNDERSTT_MODELS_DIR", "")

	dir := DefaultCacheDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}
	want := filepath.Join(home, ".cache", "thunderstt", "models")
	if dir != want {
		t.Errorf("DefaultCacheDir() = %q, want %q", dir, want)
	}
}

func TestDefaultCacheDir_envOverride(t *testing.T) {
	custom := "/tmp/thunderstt-test-models-override"
	t.Setenv("THUNDERSTT_MODELS_DIR", custom)

	dir := DefaultCacheDir()
	if dir != custom {
		t.Errorf("DefaultCacheDir() = %q, want %q (from env)", dir, custom)
	}
}

func TestModelPath(t *testing.T) {
	// Use a known cache dir to make assertion deterministic.
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	got := ModelPath("parakeet-tdt-0.6b-v3")
	want := filepath.Join(tmpDir, "parakeet-tdt-0.6b-v3")
	if got != want {
		t.Errorf("ModelPath() = %q, want %q", got, want)
	}
}

func TestIsModelCached_notCached(t *testing.T) {
	// Point cache dir to a fresh temp directory with no model files.
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	if IsModelCached("parakeet-tdt-0.6b-v3") {
		t.Error("IsModelCached() = true for model that does not exist on disk, want false")
	}
}

func TestIsModelCached_unknownModel(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	if IsModelCached("nonexistent-model") {
		t.Error("IsModelCached() = true for unknown model, want false")
	}
}

func TestIsModelCached_complete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	modelID := "parakeet-tdt-0.6b-v3"
	info, err := GetModel(modelID)
	if err != nil {
		t.Fatalf("GetModel(%q): %v", modelID, err)
	}

	modelDir := filepath.Join(tmpDir, modelID)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create all required files with non-zero content.
	for _, f := range info.Files {
		path := filepath.Join(modelDir, f)
		if err := os.WriteFile(path, []byte("fake-model-data"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	if !IsModelCached(modelID) {
		t.Error("IsModelCached() = false for fully cached model, want true")
	}
}

func TestIsModelCached_incomplete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	modelID := "parakeet-tdt-0.6b-v3"
	info, err := GetModel(modelID)
	if err != nil {
		t.Fatalf("GetModel(%q): %v", modelID, err)
	}

	modelDir := filepath.Join(tmpDir, modelID)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create only the first file, leaving the rest missing.
	if len(info.Files) < 2 {
		t.Skip("model has fewer than 2 required files; cannot test incomplete cache")
	}
	firstFile := filepath.Join(modelDir, info.Files[0])
	if err := os.WriteFile(firstFile, []byte("fake-model-data"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", firstFile, err)
	}

	if IsModelCached(modelID) {
		t.Error("IsModelCached() = true for incomplete model, want false")
	}
}

func TestIsModelCached_emptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	modelID := "parakeet-tdt-0.6b-v3"
	info, err := GetModel(modelID)
	if err != nil {
		t.Fatalf("GetModel(%q): %v", modelID, err)
	}

	modelDir := filepath.Join(tmpDir, modelID)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create all files, but make one of them zero-length.
	for i, f := range info.Files {
		path := filepath.Join(modelDir, f)
		content := []byte("fake-model-data")
		if i == 0 {
			content = []byte{} // zero-length
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	if IsModelCached(modelID) {
		t.Error("IsModelCached() = true when a file is zero-length, want false")
	}
}

func TestEnsureModel_unknownModel(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THUNDERSTT_MODELS_DIR", tmpDir)

	_, err := EnsureModel("nonexistent-model")
	if err == nil {
		t.Fatal("EnsureModel(\"nonexistent-model\") expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown model") {
		t.Errorf("error = %q, want it to contain \"unknown model\"", err.Error())
	}
}
