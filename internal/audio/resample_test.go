//go:build !cgo

package audio

import (
	"math"
	"testing"
)

func TestResample_SameRate(t *testing.T) {
	input := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	out := Resample(input, 16000, 16000)

	// When rates are equal, the original slice is returned (no copy).
	if &out[0] != &input[0] {
		t.Error("expected same slice when fromRate == toRate")
	}
	if len(out) != len(input) {
		t.Errorf("expected len %d, got %d", len(input), len(out))
	}
}

func TestResample_EmptyInput(t *testing.T) {
	out := Resample(nil, 48000, 16000)
	if out != nil {
		t.Errorf("expected nil for nil input, got %v", out)
	}

	out = Resample([]float32{}, 48000, 16000)
	if len(out) != 0 {
		t.Errorf("expected empty output for empty input, got len %d", len(out))
	}
}

func TestResample_48kTo16k(t *testing.T) {
	// Generate a 48kHz signal: 480 samples = 10ms of audio.
	const fromRate = 48000
	const toRate = 16000
	const numSamples = 480

	input := make([]float32, numSamples)
	// Fill with a simple sine wave at 1kHz.
	for i := range input {
		input[i] = float32(math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(fromRate)))
	}

	out := Resample(input, fromRate, toRate)

	// Expected output length: 480 * (16000/48000) = 160
	expectedLen := int(float64(numSamples) * float64(toRate) / float64(fromRate))
	if len(out) != expectedLen {
		t.Errorf("expected output length %d, got %d", expectedLen, len(out))
	}

	// Verify the output values are within a valid range.
	for i, v := range out {
		if v < -1.1 || v > 1.1 {
			t.Errorf("sample[%d] = %f is out of range [-1.1, 1.1]", i, v)
		}
	}
}

func TestResample_Upsample(t *testing.T) {
	// 8kHz -> 16kHz should double the number of samples.
	input := []float32{0.0, 1.0, 0.0, -1.0}
	out := Resample(input, 8000, 16000)

	expectedLen := int(float64(len(input)) * (16000.0 / 8000.0))
	if len(out) != expectedLen {
		t.Errorf("expected output length %d, got %d", expectedLen, len(out))
	}

	// First sample should be close to original first sample.
	if math.Abs(float64(out[0])) > 0.01 {
		t.Errorf("first output sample should be ~0.0, got %f", out[0])
	}
}

func TestToMono_SingleChannel(t *testing.T) {
	input := []float32{0.1, 0.2, 0.3}
	out := ToMono(input, 1)

	// With 1 channel, should return the same slice.
	if &out[0] != &input[0] {
		t.Error("expected same slice for mono input")
	}
}

func TestToMono_ZeroChannels(t *testing.T) {
	input := []float32{0.1, 0.2, 0.3}
	out := ToMono(input, 0)

	// channels <= 1 returns the input unchanged.
	if &out[0] != &input[0] {
		t.Error("expected same slice for 0 channels")
	}
}

func TestToMono_Stereo(t *testing.T) {
	// Stereo: L, R, L, R, L, R
	// Frame 0: L=0.4, R=0.6 -> avg=0.5
	// Frame 1: L=1.0, R=-1.0 -> avg=0.0
	// Frame 2: L=0.2, R=0.8 -> avg=0.5
	input := []float32{0.4, 0.6, 1.0, -1.0, 0.2, 0.8}
	out := ToMono(input, 2)

	if len(out) != 3 {
		t.Fatalf("expected 3 mono samples, got %d", len(out))
	}

	expected := []float32{0.5, 0.0, 0.5}
	for i, want := range expected {
		if diff := math.Abs(float64(out[i] - want)); diff > 1e-6 {
			t.Errorf("mono[%d]: want %f, got %f", i, want, out[i])
		}
	}
}

func TestToMono_ThreeChannels(t *testing.T) {
	// 3-channel interleaved: each frame has 3 samples.
	// Frame 0: 0.3, 0.6, 0.9 -> avg = 0.6
	// Frame 1: -0.3, -0.6, -0.9 -> avg = -0.6
	input := []float32{0.3, 0.6, 0.9, -0.3, -0.6, -0.9}
	out := ToMono(input, 3)

	if len(out) != 2 {
		t.Fatalf("expected 2 mono frames, got %d", len(out))
	}

	expected := []float32{0.6, -0.6}
	for i, want := range expected {
		if diff := math.Abs(float64(out[i] - want)); diff > 1e-6 {
			t.Errorf("mono[%d]: want %f, got %f", i, want, out[i])
		}
	}
}

func TestToMono_EmptyInput(t *testing.T) {
	out := ToMono(nil, 2)
	if out != nil {
		t.Errorf("expected nil for nil input, got %v", out)
	}

	out = ToMono([]float32{}, 2)
	if len(out) != 0 {
		t.Errorf("expected empty for empty input, got len %d", len(out))
	}
}

func TestToMono_IncompleteFrame(t *testing.T) {
	// 5 samples with 2 channels: only 2 complete frames (4 samples); last sample discarded.
	input := []float32{0.2, 0.4, 0.6, 0.8, 0.99}
	out := ToMono(input, 2)

	if len(out) != 2 {
		t.Fatalf("expected 2 mono frames (trailing sample discarded), got %d", len(out))
	}
}
