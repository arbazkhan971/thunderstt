package pipeline

import (
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
)

func BenchmarkChunkSpeechSegments(b *testing.B) {
	segments := []SpeechSegment{
		{Start: 0, End: 120}, // 2 minutes of audio
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ChunkSpeechSegments(segments, 20.0)
	}
}

func BenchmarkStitchResults_small(b *testing.B) {
	chunks := make([]ChunkResult, 3)
	for i := range chunks {
		chunks[i] = ChunkResult{
			Offset: float64(i) * 20,
			Result: &engine.Result{
				Language: "en",
				Duration: 20,
				Segments: []engine.Segment{
					{ID: 0, Start: 0, End: 20, Text: "hello world this is a test"},
				},
			},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StitchResults(chunks)
	}
}

func BenchmarkStitchResults_large(b *testing.B) {
	// 100 chunks (simulating ~33 minutes of audio)
	chunks := make([]ChunkResult, 100)
	for i := range chunks {
		chunks[i] = ChunkResult{
			Offset: float64(i) * 20,
			Result: &engine.Result{
				Language: "en",
				Duration: 20,
				Segments: []engine.Segment{
					{ID: 0, Start: 0, End: 10, Text: "first half of chunk"},
					{ID: 1, Start: 10, End: 20, Text: "second half of chunk"},
				},
			},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StitchResults(chunks)
	}
}
