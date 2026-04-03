//go:build cgo

// This file wraps Silero VAD via the sherpa-onnx-go bindings. It requires
// CGo because sherpa-onnx-go links against native C/C++ libraries.
//
// Build without CGo (CGO_ENABLED=0) to exclude this file; the rest of the
// pipeline will still compile and work -- VAD simply won't be available.

package pipeline

import (
	"fmt"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// VAD wraps a Silero voice-activity detector backed by sherpa-onnx.
type VAD struct {
	detector *sherpa.VoiceActivityDetector
	config   sherpa.VadModelConfig
}

// SpeechSegment represents a contiguous region of detected speech.
type SpeechSegment struct {
	Start float64 // Start time in seconds.
	End   float64 // End time in seconds.
}

// NewVAD loads the Silero VAD ONNX model from modelPath and returns an
// initialised VAD instance. modelPath should point to the
// silero_vad.onnx file.
func NewVAD(modelPath string) (*VAD, error) {
	config := sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:           modelPath,
			Threshold:       0.5,
			MinSilenceDuration: 0.5,
			MinSpeechDuration:  0.25,
			WindowSize:      512,
		},
		SampleRate:    16000,
		NumThreads:    1,
		Debug:         0,
		Provider:      "cpu",
	}

	detector := sherpa.NewVoiceActivityDetector(&config, 30.0) // 30s buffer
	if detector == nil {
		return nil, fmt.Errorf("vad: failed to create voice activity detector from %s", modelPath)
	}

	return &VAD{
		detector: detector,
		config:   config,
	}, nil
}

// DetectSpeech runs the Silero VAD over the provided 16 kHz mono audio and
// returns time-stamped speech segments. The audio must already be at the
// sample rate configured in the VAD (16000 by default).
func (v *VAD) DetectSpeech(audioSamples []float32, sampleRate int) ([]SpeechSegment, error) {
	if sampleRate != int(v.config.SampleRate) {
		return nil, fmt.Errorf("vad: expected %d Hz audio, got %d Hz", v.config.SampleRate, sampleRate)
	}

	windowSize := int(v.config.SileroVad.WindowSize)
	if windowSize <= 0 {
		windowSize = 512
	}

	// Feed audio to the detector in windows.
	for i := 0; i+windowSize <= len(audioSamples); i += windowSize {
		window := audioSamples[i : i+windowSize]
		v.detector.AcceptWaveform(window)
	}

	// Handle any remaining samples shorter than a full window by zero-padding.
	remainder := len(audioSamples) % windowSize
	if remainder > 0 {
		padded := make([]float32, windowSize)
		copy(padded, audioSamples[len(audioSamples)-remainder:])
		v.detector.AcceptWaveform(padded)
	}

	// Flush to ensure the detector emits any trailing segment.
	v.detector.Flush()

	// Collect speech segments.
	var segments []SpeechSegment
	for !v.detector.IsEmpty() {
		seg := v.detector.Front()
		v.detector.Pop()

		startSec := float64(seg.Start) / float64(sampleRate)
		// seg.Samples contains the speech audio; derive the end time.
		endSec := startSec + float64(len(seg.Samples))/float64(sampleRate)

		segments = append(segments, SpeechSegment{
			Start: startSec,
			End:   endSec,
		})
	}

	// Reset the detector state for the next call.
	v.detector.Clear()

	return segments, nil
}

// Close releases native resources held by the VAD detector.
func (v *VAD) Close() {
	if v.detector != nil {
		sherpa.DeleteVoiceActivityDetector(v.detector)
		v.detector = nil
	}
}
