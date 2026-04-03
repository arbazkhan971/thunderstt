package engine

import (
	"errors"
	"fmt"
	"sort"
	"testing"
)

// mockEngine is a configurable Engine implementation for testing AutoEngine.
type mockEngine struct {
	name      string
	languages []string
	result    *Result
	err       error
	closed    bool
}

func (m *mockEngine) Transcribe(audio []float32, sampleRate int, opts Options) (*Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &Result{}, nil
}

func (m *mockEngine) SupportedLanguages() []string { return m.languages }
func (m *mockEngine) ModelName() string            { return m.name }
func (m *mockEngine) Close() error {
	m.closed = true
	return nil
}

// Compile-time check.
var _ Engine = (*mockEngine)(nil)

// mockEngineWithCloseErr returns an error on Close.
type mockEngineWithCloseErr struct {
	mockEngine
	closeErr error
}

func (m *mockEngineWithCloseErr) Close() error {
	m.closed = true
	return m.closeErr
}

func TestNewAutoEngine_nilPrimary(t *testing.T) {
	fallback := &mockEngine{name: "fallback"}
	_, err := NewAutoEngine(nil, fallback)
	if err == nil {
		t.Fatal("NewAutoEngine(nil, fallback) expected error, got nil")
	}
}

func TestNewAutoEngine_nilFallback(t *testing.T) {
	primary := &mockEngine{name: "primary"}
	_, err := NewAutoEngine(primary, nil)
	if err == nil {
		t.Fatal("NewAutoEngine(primary, nil) expected error, got nil")
	}
}

func TestAutoEngine_Transcribe_withLanguageHint_primary(t *testing.T) {
	primary := &mockEngine{
		name:      "primary",
		languages: []string{"en"},
		result: &Result{
			Language: "en",
			Segments: []Segment{{Text: "primary result"}},
		},
	}
	fallback := &mockEngine{
		name:      "fallback",
		languages: []string{"en", "de"},
		result: &Result{
			Language: "en",
			Segments: []Segment{{Text: "fallback result"}},
		},
	}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	audio := []float32{0.1, 0.2}
	res, err := ae.Transcribe(audio, 16000, Options{Language: "en"})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got := res.FullText(); got != "primary result" {
		t.Errorf("Transcribe() = %q, want %q", got, "primary result")
	}
}

func TestAutoEngine_Transcribe_withLanguageHint_fallback(t *testing.T) {
	primary := &mockEngine{
		name:      "primary",
		languages: []string{"en"},
	}
	fallback := &mockEngine{
		name:      "fallback",
		languages: []string{"de", "fr"},
		result: &Result{
			Language: "de",
			Segments: []Segment{{Text: "fallback deutsch"}},
		},
	}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	audio := []float32{0.1, 0.2}
	res, err := ae.Transcribe(audio, 16000, Options{Language: "de"})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got := res.FullText(); got != "fallback deutsch" {
		t.Errorf("Transcribe() = %q, want %q", got, "fallback deutsch")
	}
}

func TestAutoEngine_Transcribe_unsupportedLanguage(t *testing.T) {
	primary := &mockEngine{name: "primary", languages: []string{"en"}}
	fallback := &mockEngine{name: "fallback", languages: []string{"de"}}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	audio := []float32{0.1}
	_, err = ae.Transcribe(audio, 16000, Options{Language: "ja"})
	if err == nil {
		t.Fatal("expected error for unsupported language, got nil")
	}
	var unsup *ErrUnsupportedLanguage
	if !errors.As(err, &unsup) {
		t.Errorf("error type = %T, want *ErrUnsupportedLanguage", err)
	}
}

func TestAutoEngine_Transcribe_noHint_primarySucceeds(t *testing.T) {
	primary := &mockEngine{
		name:      "primary",
		languages: []string{"en"},
		result: &Result{
			Language: "en",
			Segments: []Segment{{Text: "primary wins"}},
		},
	}
	fallback := &mockEngine{
		name:      "fallback",
		languages: []string{"en", "de"},
		result: &Result{
			Language: "en",
			Segments: []Segment{{Text: "fallback text"}},
		},
	}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	audio := []float32{0.1}
	res, err := ae.Transcribe(audio, 16000, Options{})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got := res.FullText(); got != "primary wins" {
		t.Errorf("Transcribe() = %q, want %q", got, "primary wins")
	}
}

func TestAutoEngine_Transcribe_noHint_primaryEmpty_fallbackSucceeds(t *testing.T) {
	primary := &mockEngine{
		name:      "primary",
		languages: []string{"en"},
		result:    &Result{}, // empty result
	}
	fallback := &mockEngine{
		name:      "fallback",
		languages: []string{"en", "de"},
		result: &Result{
			Language: "de",
			Segments: []Segment{{Text: "fallback wins"}},
		},
	}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	audio := []float32{0.1}
	res, err := ae.Transcribe(audio, 16000, Options{})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got := res.FullText(); got != "fallback wins" {
		t.Errorf("Transcribe() = %q, want %q", got, "fallback wins")
	}
}

func TestAutoEngine_Transcribe_emptyAudio(t *testing.T) {
	primary := &mockEngine{name: "primary", languages: []string{"en"}}
	fallback := &mockEngine{name: "fallback", languages: []string{"de"}}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ae.Transcribe([]float32{}, 16000, Options{})
	if !errors.Is(err, ErrEmptyAudio) {
		t.Errorf("Transcribe() error = %v, want ErrEmptyAudio", err)
	}

	// Also test nil audio slice.
	_, err = ae.Transcribe(nil, 16000, Options{})
	if !errors.Is(err, ErrEmptyAudio) {
		t.Errorf("Transcribe(nil) error = %v, want ErrEmptyAudio", err)
	}
}

func TestAutoEngine_SupportedLanguages(t *testing.T) {
	primary := &mockEngine{name: "primary", languages: []string{"en"}}
	fallback := &mockEngine{name: "fallback", languages: []string{"en", "de", "fr"}}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	langs := ae.SupportedLanguages()

	// Should be the union: en, de, fr (deduplicated).
	sort.Strings(langs)
	want := []string{"de", "en", "fr"}
	if len(langs) != len(want) {
		t.Fatalf("SupportedLanguages() = %v, want %v", langs, want)
	}
	for i := range want {
		if langs[i] != want[i] {
			t.Errorf("SupportedLanguages()[%d] = %q, want %q", i, langs[i], want[i])
		}
	}
}

func TestAutoEngine_ModelName(t *testing.T) {
	primary := &mockEngine{name: "parakeet"}
	fallback := &mockEngine{name: "whisper"}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	want := "auto(parakeet+whisper)"
	if got := ae.ModelName(); got != want {
		t.Errorf("ModelName() = %q, want %q", got, want)
	}
}

func TestAutoEngine_Close(t *testing.T) {
	primary := &mockEngine{name: "primary"}
	fallback := &mockEngine{name: "fallback"}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	if err := ae.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	if !primary.closed {
		t.Error("primary engine was not closed")
	}
	if !fallback.closed {
		t.Error("fallback engine was not closed")
	}
}

func TestAutoEngine_Close_withErrors(t *testing.T) {
	primary := &mockEngineWithCloseErr{
		mockEngine: mockEngine{name: "primary"},
		closeErr:   fmt.Errorf("primary close error"),
	}
	fallback := &mockEngineWithCloseErr{
		mockEngine: mockEngine{name: "fallback"},
		closeErr:   fmt.Errorf("fallback close error"),
	}

	ae, err := NewAutoEngine(primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	err = ae.Close()
	if err == nil {
		t.Fatal("Close() expected error when sub-engines fail, got nil")
	}
	if !primary.closed {
		t.Error("primary engine was not closed")
	}
	if !fallback.closed {
		t.Error("fallback engine was not closed")
	}
}
