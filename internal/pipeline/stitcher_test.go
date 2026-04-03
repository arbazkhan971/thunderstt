//go:build !cgo

package pipeline

import (
	"math"
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
)

func TestStitchResults_Empty(t *testing.T) {
	result := StitchResults(nil)
	if result == nil {
		t.Fatal("StitchResults(nil) should return non-nil empty result")
	}
	if len(result.Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(result.Segments))
	}
	if result.Duration != 0 {
		t.Errorf("expected duration 0, got %f", result.Duration)
	}

	result = StitchResults([]ChunkResult{})
	if result == nil {
		t.Fatal("StitchResults([]) should return non-nil empty result")
	}
	if len(result.Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(result.Segments))
	}
}

func TestStitchResults_SingleChunk(t *testing.T) {
	chunks := []ChunkResult{
		{
			Offset: 0.0,
			Result: &engine.Result{
				Language:     "en",
				LanguageProb: 0.95,
				Segments: []engine.Segment{
					{
						ID:    0,
						Start: 0.0,
						End:   3.0,
						Text:  "Hello world",
						Words: []engine.Word{
							{Word: "Hello", Start: 0.0, End: 1.5, Prob: 0.99},
							{Word: "world", Start: 1.6, End: 3.0, Prob: 0.98},
						},
					},
				},
			},
		},
	}

	result := StitchResults(chunks)

	if result.Language != "en" {
		t.Errorf("language = %q, want %q", result.Language, "en")
	}
	if result.LanguageProb != 0.95 {
		t.Errorf("languageProb = %f, want 0.95", result.LanguageProb)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}

	seg := result.Segments[0]
	if seg.ID != 0 {
		t.Errorf("segment ID = %d, want 0", seg.ID)
	}
	if seg.Start != 0.0 || seg.End != 3.0 {
		t.Errorf("segment times = {%.1f, %.1f}, want {0.0, 3.0}", seg.Start, seg.End)
	}
	if seg.Text != "Hello world" {
		t.Errorf("segment text = %q, want %q", seg.Text, "Hello world")
	}
	if len(seg.Words) != 2 {
		t.Errorf("expected 2 words, got %d", len(seg.Words))
	}
}

func TestStitchResults_TwoChunks_OffsetAdjustment(t *testing.T) {
	chunks := []ChunkResult{
		{
			Offset: 0.0,
			Result: &engine.Result{
				Language:     "en",
				LanguageProb: 0.95,
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 5.0, Text: "First chunk"},
				},
			},
		},
		{
			Offset: 10.0, // Second chunk starts at 10s in the original audio.
			Result: &engine.Result{
				Language: "en",
				Segments: []engine.Segment{
					{
						ID:    0, // Local ID within chunk, should be re-numbered.
						Start: 0.0,
						End:   3.0,
						Text:  "Second chunk",
						Words: []engine.Word{
							{Word: "Second", Start: 0.0, End: 1.5, Prob: 0.9},
							{Word: "chunk", Start: 1.6, End: 3.0, Prob: 0.8},
						},
					},
				},
			},
		},
	}

	result := StitchResults(chunks)

	if len(result.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(result.Segments))
	}

	// First segment: no offset adjustment.
	seg0 := result.Segments[0]
	if seg0.ID != 0 {
		t.Errorf("segment[0].ID = %d, want 0", seg0.ID)
	}
	if seg0.Start != 0.0 || seg0.End != 5.0 {
		t.Errorf("segment[0] times = {%.1f, %.1f}, want {0.0, 5.0}", seg0.Start, seg0.End)
	}

	// Second segment: times shifted by offset 10.0, ID renumbered.
	seg1 := result.Segments[1]
	if seg1.ID != 1 {
		t.Errorf("segment[1].ID = %d, want 1", seg1.ID)
	}
	if math.Abs(seg1.Start-10.0) > 0.001 || math.Abs(seg1.End-13.0) > 0.001 {
		t.Errorf("segment[1] times = {%.1f, %.1f}, want {10.0, 13.0}", seg1.Start, seg1.End)
	}

	// Word timestamps should also be shifted.
	if len(seg1.Words) != 2 {
		t.Fatalf("segment[1] expected 2 words, got %d", len(seg1.Words))
	}
	if math.Abs(seg1.Words[0].Start-10.0) > 0.001 {
		t.Errorf("word[0].Start = %.1f, want 10.0", seg1.Words[0].Start)
	}
	if math.Abs(seg1.Words[0].End-11.5) > 0.001 {
		t.Errorf("word[0].End = %.1f, want 11.5", seg1.Words[0].End)
	}
	if math.Abs(seg1.Words[1].Start-11.6) > 0.001 {
		t.Errorf("word[1].Start = %.1f, want 11.6", seg1.Words[1].Start)
	}
	if math.Abs(seg1.Words[1].End-13.0) > 0.001 {
		t.Errorf("word[1].End = %.1f, want 13.0", seg1.Words[1].End)
	}

	// Duration should be max end time (13.0).
	if math.Abs(result.Duration-13.0) > 0.001 {
		t.Errorf("duration = %.1f, want 13.0", result.Duration)
	}
}

func TestStitchResults_SegmentIDRenumbering(t *testing.T) {
	chunks := []ChunkResult{
		{
			Offset: 0.0,
			Result: &engine.Result{
				Language: "de",
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 2.0, Text: "A"},
					{ID: 1, Start: 2.5, End: 4.0, Text: "B"},
				},
			},
		},
		{
			Offset: 5.0,
			Result: &engine.Result{
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 1.0, Text: "C"},
					{ID: 1, Start: 1.5, End: 3.0, Text: "D"},
					{ID: 2, Start: 3.5, End: 5.0, Text: "E"},
				},
			},
		},
	}

	result := StitchResults(chunks)

	if len(result.Segments) != 5 {
		t.Fatalf("expected 5 segments, got %d", len(result.Segments))
	}

	// IDs should be 0, 1, 2, 3, 4.
	for i, seg := range result.Segments {
		if seg.ID != i {
			t.Errorf("segment[%d].ID = %d, want %d", i, seg.ID, i)
		}
	}

	// Language should come from first chunk.
	if result.Language != "de" {
		t.Errorf("language = %q, want %q", result.Language, "de")
	}
}

func TestStitchResults_NilResultInChunk(t *testing.T) {
	chunks := []ChunkResult{
		{Offset: 0.0, Result: nil}, // nil result should be skipped
		{
			Offset: 5.0,
			Result: &engine.Result{
				Language: "en",
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 2.0, Text: "Hello"},
				},
			},
		},
	}

	result := StitchResults(chunks)

	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].ID != 0 {
		t.Errorf("segment ID = %d, want 0", result.Segments[0].ID)
	}
	if result.Language != "en" {
		t.Errorf("language = %q, want %q", result.Language, "en")
	}
}

func TestStitchResults_LanguageFromFirstNonEmpty(t *testing.T) {
	chunks := []ChunkResult{
		{
			Offset: 0.0,
			Result: &engine.Result{
				Language: "", // Empty language
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 1.0, Text: "A"},
				},
			},
		},
		{
			Offset: 2.0,
			Result: &engine.Result{
				Language:     "fr",
				LanguageProb: 0.88,
				Segments: []engine.Segment{
					{ID: 0, Start: 0.0, End: 1.0, Text: "B"},
				},
			},
		},
	}

	result := StitchResults(chunks)
	if result.Language != "fr" {
		t.Errorf("language = %q, want %q", result.Language, "fr")
	}
	if result.LanguageProb != 0.88 {
		t.Errorf("languageProb = %f, want 0.88", result.LanguageProb)
	}
}
