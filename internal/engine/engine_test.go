package engine

import "testing"

func TestResult_FullText(t *testing.T) {
	r := &Result{
		Segments: []Segment{
			{Text: "hello"},
			{Text: "world"},
			{Text: "foo"},
		},
	}
	got := r.FullText()
	want := "hello world foo"
	if got != want {
		t.Errorf("FullText() = %q, want %q", got, want)
	}
}

func TestResult_FullText_empty(t *testing.T) {
	// nil receiver
	var r *Result
	if got := r.FullText(); got != "" {
		t.Errorf("FullText() on nil = %q, want empty", got)
	}

	// non-nil but no segments
	r2 := &Result{}
	if got := r2.FullText(); got != "" {
		t.Errorf("FullText() on empty result = %q, want empty", got)
	}
}

func TestResult_IsEmpty(t *testing.T) {
	// nil receiver
	var r *Result
	if !r.IsEmpty() {
		t.Error("IsEmpty() on nil should be true")
	}

	// no segments
	r2 := &Result{}
	if !r2.IsEmpty() {
		t.Error("IsEmpty() with no segments should be true")
	}

	// segments with empty text
	r3 := &Result{
		Segments: []Segment{
			{Text: ""},
			{Text: ""},
		},
	}
	if !r3.IsEmpty() {
		t.Error("IsEmpty() with all empty-text segments should be true")
	}
}

func TestResult_IsEmpty_withText(t *testing.T) {
	r := &Result{
		Segments: []Segment{
			{Text: ""},
			{Text: "hello"},
		},
	}
	if r.IsEmpty() {
		t.Error("IsEmpty() should be false when a segment has text")
	}
}
