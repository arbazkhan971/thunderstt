package engine

import "testing"

func TestNoopEngine_Transcribe(t *testing.T) {
	e := NewNoopEngine("test-model")
	audio := []float32{0.1, 0.2, 0.3}
	result, err := e.Transcribe(audio, 16000, Options{Language: "en"})
	if err != nil {
		t.Fatalf("Transcribe() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Transcribe() returned nil result")
	}
	if !result.IsEmpty() {
		t.Error("Transcribe() result should be empty")
	}
	if result.Language != "en" {
		t.Errorf("Transcribe() result.Language = %q, want %q", result.Language, "en")
	}
}

func TestNoopEngine_ModelName(t *testing.T) {
	name := "my-noop-model"
	e := NewNoopEngine(name)
	if got := e.ModelName(); got != name {
		t.Errorf("ModelName() = %q, want %q", got, name)
	}
}

func TestNoopEngine_SupportedLanguages(t *testing.T) {
	e := NewNoopEngine("test")
	if langs := e.SupportedLanguages(); langs != nil {
		t.Errorf("SupportedLanguages() = %v, want nil", langs)
	}
}

func TestNoopEngine_Close(t *testing.T) {
	e := NewNoopEngine("test")
	if err := e.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
