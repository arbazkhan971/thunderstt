package audio

import (
	"testing"
)

func BenchmarkResample_16kto16k(b *testing.B) {
	samples := make([]float32, 16000) // 1 second at 16kHz
	for i := range samples {
		samples[i] = float32(i) / float32(len(samples))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resample(samples, 16000, 16000)
	}
}

func BenchmarkResample_44k1to16k(b *testing.B) {
	samples := make([]float32, 44100) // 1 second at 44.1kHz
	for i := range samples {
		samples[i] = float32(i) / float32(len(samples))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resample(samples, 44100, 16000)
	}
}

func BenchmarkToMono_stereo(b *testing.B) {
	// 1 second stereo at 16kHz
	samples := make([]float32, 32000)
	for i := range samples {
		samples[i] = float32(i) / float32(len(samples))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToMono(samples, 2)
	}
}

func BenchmarkResample_longAudio(b *testing.B) {
	// 60 seconds at 44.1kHz
	samples := make([]float32, 44100*60)
	for i := range samples {
		samples[i] = float32(i%1000) / 1000
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resample(samples, 44100, 16000)
	}
}
