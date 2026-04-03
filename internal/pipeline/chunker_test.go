//go:build !cgo

package pipeline

import (
	"math"
	"testing"
)

func TestChunkSpeechSegments_Empty(t *testing.T) {
	chunks := ChunkSpeechSegments(nil, 20.0)
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}

	chunks = ChunkSpeechSegments([]SpeechSegment{}, 20.0)
	if chunks != nil {
		t.Errorf("expected nil for empty slice, got %v", chunks)
	}
}

func TestChunkSpeechSegments_SingleSegment(t *testing.T) {
	segments := []SpeechSegment{
		{Start: 0.0, End: 5.0},
	}
	chunks := ChunkSpeechSegments(segments, 20.0)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != 0.0 || chunks[0].End != 5.0 {
		t.Errorf("chunk = {%.1f, %.1f}, want {0.0, 5.0}", chunks[0].Start, chunks[0].End)
	}
}

func TestChunkSpeechSegments_AllFitInOneChunk(t *testing.T) {
	segments := []SpeechSegment{
		{Start: 0.0, End: 3.0},
		{Start: 3.5, End: 6.0},
		{Start: 7.0, End: 10.0},
	}
	// Max duration 20s - all segments fit (total span: 0.0 to 10.0 = 10s).
	chunks := ChunkSpeechSegments(segments, 20.0)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != 0.0 || chunks[0].End != 10.0 {
		t.Errorf("chunk = {%.1f, %.1f}, want {0.0, 10.0}", chunks[0].Start, chunks[0].End)
	}
}

func TestChunkSpeechSegments_NeedsSplitting(t *testing.T) {
	segments := []SpeechSegment{
		{Start: 0.0, End: 5.0},
		{Start: 6.0, End: 12.0},
		{Start: 13.0, End: 18.0},
		{Start: 20.0, End: 25.0},
	}
	// Max duration 15s.
	// Chunk 1: seg[0] + seg[1] = 0..12 (12s fits)
	//   + seg[2] would make 0..18 = 18s > 15s, so close chunk 1 at 12.0.
	// Chunk 2: seg[2] = 13..18 (5s fits)
	//   + seg[3] would make 13..25 = 12s < 15s, so include.
	// Chunk 2: 13..25 (12s)
	chunks := ChunkSpeechSegments(segments, 15.0)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if chunks[0].Start != 0.0 || chunks[0].End != 12.0 {
		t.Errorf("chunk[0] = {%.1f, %.1f}, want {0.0, 12.0}", chunks[0].Start, chunks[0].End)
	}
	if chunks[1].Start != 13.0 || chunks[1].End != 25.0 {
		t.Errorf("chunk[1] = {%.1f, %.1f}, want {13.0, 25.0}", chunks[1].Start, chunks[1].End)
	}
}

func TestChunkSpeechSegments_LargeSegmentGetsOwnChunk(t *testing.T) {
	segments := []SpeechSegment{
		{Start: 0.0, End: 5.0},
		{Start: 6.0, End: 30.0}, // 24s > maxDuration, gets own chunk
		{Start: 31.0, End: 35.0},
	}
	// Max duration 10s.
	// Chunk 1: seg[0] = 0..5 (5s fits)
	//   + seg[1] would make 0..30 = 30s > 10s, so close chunk 1 at 5.0.
	// Chunk 2: seg[1] alone = 6..30 (24s, exceeds max but single segment)
	//   + seg[2] would make 6..35 = 29s > 10s, so close chunk 2 at 30.0.
	// Chunk 3: seg[2] = 31..35.
	chunks := ChunkSpeechSegments(segments, 10.0)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Start != 0.0 || chunks[0].End != 5.0 {
		t.Errorf("chunk[0] = {%.1f, %.1f}, want {0.0, 5.0}", chunks[0].Start, chunks[0].End)
	}
	if chunks[1].Start != 6.0 || chunks[1].End != 30.0 {
		t.Errorf("chunk[1] = {%.1f, %.1f}, want {6.0, 30.0}", chunks[1].Start, chunks[1].End)
	}
	if chunks[2].Start != 31.0 || chunks[2].End != 35.0 {
		t.Errorf("chunk[2] = {%.1f, %.1f}, want {31.0, 35.0}", chunks[2].Start, chunks[2].End)
	}
}

func TestChunkSpeechSegments_DefaultMaxDuration(t *testing.T) {
	// When maxDuration <= 0, it should use DefaultChunkDuration (20.0).
	segments := []SpeechSegment{
		{Start: 0.0, End: 10.0},
		{Start: 11.0, End: 19.0},
	}
	// Total span 0..19 = 19s < 20s default, so one chunk.
	chunks := ChunkSpeechSegments(segments, 0)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk with default maxDuration, got %d", len(chunks))
	}
}

func TestChunkSpeechSegments_ManySmallSegments(t *testing.T) {
	// Many 1-second segments, maxDuration = 5.
	var segments []SpeechSegment
	for i := 0; i < 20; i++ {
		s := float64(i) * 1.5
		segments = append(segments, SpeechSegment{Start: s, End: s + 1.0})
	}

	chunks := ChunkSpeechSegments(segments, 5.0)

	// Verify all chunks respect the maxDuration or contain a single segment.
	for i, c := range chunks {
		dur := c.End - c.Start
		if dur > 5.0+0.001 { // small epsilon for float comparison
			t.Errorf("chunk[%d] duration = %.2f, exceeds max 5.0", i, dur)
		}
	}

	// Verify no gaps: the union of all chunks covers all segments.
	if chunks[0].Start != segments[0].Start {
		t.Errorf("first chunk start = %.1f, want %.1f", chunks[0].Start, segments[0].Start)
	}
	lastChunk := chunks[len(chunks)-1]
	lastSeg := segments[len(segments)-1]
	if math.Abs(lastChunk.End-lastSeg.End) > 0.001 {
		t.Errorf("last chunk end = %.1f, want %.1f", lastChunk.End, lastSeg.End)
	}
}
