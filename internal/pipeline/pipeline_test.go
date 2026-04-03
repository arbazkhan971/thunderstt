//go:build !cgo

package pipeline

import (
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
)

// noopEngine is a minimal engine.Engine implementation used for unit tests.
// It returns a canned result and never fails.
type noopEngine struct {
	model string
}

func (e *noopEngine) Transcribe(_ []float32, _ int, _ engine.Options) (*engine.Result, error) {
	return &engine.Result{
		Language: "en",
		Segments: []engine.Segment{
			{ID: 0, Start: 0, End: 1, Text: "noop"},
		},
	}, nil
}

func (e *noopEngine) SupportedLanguages() []string { return nil }
func (e *noopEngine) ModelName() string            { return e.model }
func (e *noopEngine) Close() error                 { return nil }

// ---------------------------------------------------------------------------
// Pipeline constructor tests
// ---------------------------------------------------------------------------

func TestPipeline_New(t *testing.T) {
	eng := &noopEngine{model: "test-model"}
	p := New(eng)
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if !p.Ready() {
		t.Error("expected Ready() == true with a valid engine")
	}
}

func TestPipeline_New_nilEngine(t *testing.T) {
	p := New(nil)
	if p == nil {
		t.Fatal("New(nil) returned nil")
	}
	if p.Ready() {
		t.Error("expected Ready() == false when engine is nil")
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestPipeline_Close(t *testing.T) {
	eng := &noopEngine{model: "test-model"}
	p := New(eng)

	// Close should not panic and should clear the ready flag.
	if err := p.Close(); err != nil {
		t.Fatalf("Close() returned unexpected error: %v", err)
	}
	if p.Ready() {
		t.Error("expected Ready() == false after Close()")
	}

	// Closing a pipeline with a nil engine should also be safe.
	p2 := New(nil)
	if err := p2.Close(); err != nil {
		t.Fatalf("Close() on nil-engine pipeline returned unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ProcessAudio edge case
// ---------------------------------------------------------------------------

func TestPipeline_ProcessAudio_empty(t *testing.T) {
	eng := &noopEngine{model: "test-model"}
	p := New(eng)

	// An empty audio slice should still be accepted (the noop engine handles
	// it). The main assertion is that the pipeline does not panic.
	result, err := p.ProcessAudio([]float32{}, 16000)
	if err != nil {
		t.Fatalf("ProcessAudio(empty) returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("ProcessAudio(empty) returned nil result")
	}
}

// ---------------------------------------------------------------------------
// ModelName
// ---------------------------------------------------------------------------

func TestPipeline_ModelName(t *testing.T) {
	eng := &noopEngine{model: "whisper-large-v3"}
	p := New(eng)
	got := p.ModelName()
	if got != "whisper-large-v3" {
		t.Errorf("ModelName() = %q, want %q", got, "whisper-large-v3")
	}
}

func TestPipeline_ModelName_nilEngine(t *testing.T) {
	p := New(nil)
	got := p.ModelName()
	if got != "" {
		t.Errorf("ModelName() with nil engine = %q, want empty string", got)
	}
}
