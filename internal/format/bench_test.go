package format

import (
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
)

func makeResult() *engine.Result {
	segs := make([]engine.Segment, 50)
	for i := range segs {
		segs[i] = engine.Segment{
			ID:    i,
			Start: float64(i) * 5,
			End:   float64(i)*5 + 5,
			Text:  "This is a test segment with some typical text content.",
			Words: []engine.Word{
				{Word: "This", Start: float64(i) * 5, End: float64(i)*5 + 0.5},
				{Word: "is", Start: float64(i)*5 + 0.5, End: float64(i)*5 + 0.8},
				{Word: "a", Start: float64(i)*5 + 0.8, End: float64(i)*5 + 1.0},
				{Word: "test", Start: float64(i)*5 + 1.0, End: float64(i)*5 + 1.5},
			},
		}
	}
	return &engine.Result{
		Language: "en",
		Duration: 250,
		Segments: segs,
	}
}

func BenchmarkFormatResult_json(b *testing.B) {
	r := makeResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatResult(r, "json")
	}
}

func BenchmarkFormatResult_verbose_json(b *testing.B) {
	r := makeResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatResult(r, "verbose_json")
	}
}

func BenchmarkFormatResult_srt(b *testing.B) {
	r := makeResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatResult(r, "srt")
	}
}

func BenchmarkFormatResult_vtt(b *testing.B) {
	r := makeResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatResult(r, "vtt")
	}
}

func BenchmarkFormatResult_text(b *testing.B) {
	r := makeResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatResult(r, "text")
	}
}
