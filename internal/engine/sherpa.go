//go:build cgo

package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaEngine wraps a sherpa-onnx offline recognizer to implement Engine.
type SherpaEngine struct {
	recognizer *sherpa.OfflineRecognizer
	modelPath  string
	modelType  string // "parakeet" or "whisper"
	modelName  string
	vadPath    string // optional path to Silero VAD model
}

// SherpaOption is a functional option for configuring SherpaEngine.
type SherpaOption func(*SherpaEngine)

// WithVADModel sets the file path to the Silero VAD ONNX model used when
// Options.VADFilter is enabled.
func WithVADModel(path string) SherpaOption {
	return func(s *SherpaEngine) {
		s.vadPath = path
	}
}

// WithModelName overrides the human-readable model name returned by ModelName.
func WithModelName(name string) SherpaOption {
	return func(s *SherpaEngine) {
		s.modelName = name
	}
}

// NewSherpaEngine creates a SherpaEngine backed by the model files at
// modelPath. modelType must be "parakeet" or "whisper".
//
// For Parakeet models the directory is expected to contain the NeMo
// transducer ONNX files (encoder, decoder, joiner). For Whisper models the
// directory must contain the encoder and decoder ONNX files plus the
// multilingual tokens/vocabulary files.
func NewSherpaEngine(modelPath string, modelType string, options ...SherpaOption) (*SherpaEngine, error) {
	se := &SherpaEngine{
		modelPath: modelPath,
		modelType: strings.ToLower(modelType),
		modelName: filepath.Base(modelPath),
	}
	for _, opt := range options {
		opt(se)
	}

	config, err := se.buildConfig()
	if err != nil {
		return nil, fmt.Errorf("sherpa: build config: %w", err)
	}

	recognizer := sherpa.NewOfflineRecognizer(config)
	if recognizer == nil {
		return nil, fmt.Errorf("sherpa: failed to create offline recognizer for model %q", modelPath)
	}
	se.recognizer = recognizer

	return se, nil
}

// buildConfig constructs the sherpa-onnx recognizer configuration based on
// the model type.
func (se *SherpaEngine) buildConfig() (*sherpa.OfflineRecognizerConfig, error) {
	config := &sherpa.OfflineRecognizerConfig{}
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80

	switch se.modelType {
	case "parakeet":
		se.configureParakeet(config)
	case "whisper":
		se.configureWhisper(config)
	default:
		return nil, fmt.Errorf("unsupported model type %q (expected \"parakeet\" or \"whisper\")", se.modelType)
	}

	// Common settings
	config.ModelConfig.NumThreads = 4
	config.ModelConfig.Debug = 0
	config.ModelConfig.Provider = "cpu"

	return config, nil
}

// configureParakeet populates the NeMo transducer fields within the
// recognizer config for Parakeet TDT models.
func (se *SherpaEngine) configureParakeet(config *sherpa.OfflineRecognizerConfig) {
	config.ModelConfig.TransducerConfig.Encoder = filepath.Join(se.modelPath, "encoder.onnx")
	config.ModelConfig.TransducerConfig.Decoder = filepath.Join(se.modelPath, "decoder.onnx")
	config.ModelConfig.TransducerConfig.Joiner = filepath.Join(se.modelPath, "joiner.onnx")
	config.ModelConfig.ModelType = "nemo_transducer"
	config.ModelConfig.Tokens = filepath.Join(se.modelPath, "tokens.txt")
}

// configureWhisper populates the Whisper-specific fields within the
// recognizer config.
func (se *SherpaEngine) configureWhisper(config *sherpa.OfflineRecognizerConfig) {
	config.ModelConfig.Whisper.Encoder = filepath.Join(se.modelPath, "encoder.onnx")
	config.ModelConfig.Whisper.Decoder = filepath.Join(se.modelPath, "decoder.onnx")

	// TODO: confirm the exact field names for language and task once the
	// sherpa-onnx Go bindings version is pinned.
	config.ModelConfig.Whisper.Language = "en"
	config.ModelConfig.Whisper.Task = "transcribe"
	config.ModelConfig.ModelType = "whisper"
	config.ModelConfig.Tokens = se.findTokensFile()
}

// findTokensFile looks for a tokens file in the model directory.
// Whisper models use versioned names like "large-v3-turbo-tokens.txt"
// while Parakeet models use plain "tokens.txt".
func (se *SherpaEngine) findTokensFile() string {
	matches, err := filepath.Glob(filepath.Join(se.modelPath, "*-tokens.txt"))
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	return filepath.Join(se.modelPath, "tokens.txt")
}

// Transcribe implements Engine. It feeds audio samples through the sherpa-onnx
// offline recognizer pipeline and returns a structured Result.
func (se *SherpaEngine) Transcribe(audio []float32, sampleRate int, opts Options) (*Result, error) {
	if len(audio) == 0 {
		return nil, ErrEmptyAudio
	}

	// Validate language support when an explicit hint is given.
	if opts.Language != "" && !se.supportsLanguage(opts.Language) {
		return nil, &ErrUnsupportedLanguage{
			Language: opts.Language,
			Engine:   se.ModelName(),
		}
	}

	// Apply language hint for Whisper models. Parakeet is English-only so
	// the hint is ignored (already validated above).
	if se.modelType == "whisper" && opts.Language != "" {
		// TODO: sherpa-onnx does not expose per-stream language override
		// in the current Go bindings. For now we rely on the recognizer-level
		// language set during construction. A future version may rebuild the
		// recognizer or use a per-stream API.
		_ = opts.Language
	}

	// If VAD is requested and a Silero model is available, use the VAD-
	// enabled path. Otherwise fall through to plain offline recognition.
	if opts.VADFilter && se.vadPath != "" {
		return se.transcribeWithVAD(audio, sampleRate, opts)
	}

	return se.transcribeDirect(audio, sampleRate, opts)
}

// transcribeDirect runs recognition on the entire audio buffer without VAD.
func (se *SherpaEngine) transcribeDirect(audio []float32, sampleRate int, opts Options) (*Result, error) {
	stream := se.recognizer.NewStream()
	if stream == nil {
		return nil, fmt.Errorf("sherpa: failed to create offline stream")
	}
	defer stream.Destroy()

	stream.AcceptWaveform(sampleRate, audio)

	se.recognizer.Decode(stream)

	rawResult := stream.GetResult()

	durationSec := float64(len(audio)) / float64(sampleRate)

	result := &Result{
		Language: se.detectLanguage(opts.Language),
		Duration: durationSec,
	}

	text := strings.TrimSpace(rawResult.Text)
	if text == "" {
		return result, nil
	}

	seg := Segment{
		ID:    0,
		Start: 0,
		End:   durationSec,
		Text:  text,
	}

	// Populate per-word timestamps when available and requested.
	if opts.WordTimestamps && len(rawResult.Tokens) > 0 {
		seg.Words = se.extractWords(rawResult)
	}

	result.Segments = []Segment{seg}
	return result, nil
}

// transcribeWithVAD uses the Silero VAD model to split audio into speech
// segments before running recognition on each one. This reduces hallucinations
// on silent audio.
func (se *SherpaEngine) transcribeWithVAD(audio []float32, sampleRate int, opts Options) (*Result, error) {
	vadConfig := sherpa.VadModelConfig{}
	vadConfig.SileroVad.Model = se.vadPath
	vadConfig.SileroVad.Threshold = 0.5
	vadConfig.SileroVad.MinSilenceDuration = 0.5
	vadConfig.SileroVad.MinSpeechDuration = 0.25
	vadConfig.SileroVad.WindowSize = 512
	vadConfig.SampleRate = sampleRate
	vadConfig.NumThreads = 2
	vadConfig.Debug = 0
	vadConfig.Provider = "cpu"

	vad := sherpa.NewVoiceActivityDetector(&vadConfig, 30) // 30 second buffer
	if vad == nil {
		// VAD creation failed; fall back to direct transcription.
		return se.transcribeDirect(audio, sampleRate, opts)
	}
	defer vad.Delete()

	durationSec := float64(len(audio)) / float64(sampleRate)
	result := &Result{
		Language: se.detectLanguage(opts.Language),
		Duration: durationSec,
	}

	// Feed audio in chunks matching the VAD window size.
	windowSize := int(vadConfig.SileroVad.WindowSize)
	segID := 0

	for offset := 0; offset+windowSize <= len(audio); offset += windowSize {
		chunk := audio[offset : offset+windowSize]
		vad.AcceptWaveform(chunk)

		for vad.IsSpeechDetected() {
			speechSegment := vad.Front()
			vad.Pop()

			samples := speechSegment.Samples

			stream := se.recognizer.NewStream()
			if stream == nil {
				continue
			}

			stream.AcceptWaveform(sampleRate, samples)
			se.recognizer.Decode(stream)

			rawResult := stream.GetResult()
			stream.Destroy()

			text := strings.TrimSpace(rawResult.Text)
			if text == "" {
				continue
			}

			startSec := float64(speechSegment.Start) / float64(sampleRate)
			segDuration := float64(len(samples)) / float64(sampleRate)

			seg := Segment{
				ID:    segID,
				Start: startSec,
				End:   startSec + segDuration,
				Text:  text,
			}

			if opts.WordTimestamps && len(rawResult.Tokens) > 0 {
				seg.Words = se.extractWords(rawResult)
			}

			result.Segments = append(result.Segments, seg)
			segID++
		}
	}

	// Flush any remaining speech.
	vad.Flush()
	for vad.IsSpeechDetected() {
		speechSegment := vad.Front()
		vad.Pop()

		samples := speechSegment.Samples

		stream := se.recognizer.NewStream()
		if stream == nil {
			continue
		}

		stream.AcceptWaveform(sampleRate, samples)
		se.recognizer.Decode(stream)

		rawResult := stream.GetResult()
		stream.Destroy()

		text := strings.TrimSpace(rawResult.Text)
		if text == "" {
			continue
		}

		startSec := float64(speechSegment.Start) / float64(sampleRate)
		segDuration := float64(len(samples)) / float64(sampleRate)

		seg := Segment{
			ID:    segID,
			Start: startSec,
			End:   startSec + segDuration,
			Text:  text,
		}
		result.Segments = append(result.Segments, seg)
		segID++
	}

	return result, nil
}

// extractWords converts sherpa-onnx token-level output into Word structs.
// The exact availability of timestamps depends on the model and sherpa-onnx
// version.
func (se *SherpaEngine) extractWords(raw *sherpa.OfflineRecognizerResult) []Word {
	// TODO: verify the exact fields exposed by OfflineRecognizerResult for
	// token-level timestamps. The field names may differ between sherpa-onnx
	// releases. Adjust once the binding version is pinned.
	if len(raw.Tokens) == 0 {
		return nil
	}

	words := make([]Word, 0, len(raw.Tokens))
	for i, token := range raw.Tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		w := Word{
			Word: token,
		}

		// Populate timestamps if the arrays are present and aligned.
		if i < len(raw.Timestamps) {
			w.Start = float64(raw.Timestamps[i])
			// Approximate word end from next token start.
			if i+1 < len(raw.Timestamps) {
				w.End = float64(raw.Timestamps[i+1])
			}
		}

		words = append(words, w)
	}

	return words
}

// detectLanguage returns the language code that should be reported in the
// result. If the caller provided a hint we trust it; otherwise we default
// based on model type.
func (se *SherpaEngine) detectLanguage(hint string) string {
	if hint != "" {
		return hint
	}
	if se.modelType == "parakeet" {
		return "en"
	}
	// Whisper auto-detection: the recognizer handles it internally, but we
	// cannot read the detected language back from the current Go bindings.
	// Default to "en" and set LanguageProb to 0 to signal uncertainty.
	// TODO: once per-segment language detection is exposed, populate this
	// from the recognizer output.
	return "en"
}

// supportsLanguage checks whether lang is in the engine's supported set.
func (se *SherpaEngine) supportsLanguage(lang string) bool {
	lang = strings.ToLower(lang)
	for _, l := range se.SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// SupportedLanguages implements Engine.
func (se *SherpaEngine) SupportedLanguages() []string {
	switch se.modelType {
	case "parakeet":
		dst := make([]string, len(parakeetLanguages))
		copy(dst, parakeetLanguages)
		return dst
	case "whisper":
		dst := make([]string, len(whisperLanguages))
		copy(dst, whisperLanguages)
		return dst
	default:
		return nil
	}
}

// ModelName implements Engine.
func (se *SherpaEngine) ModelName() string {
	return se.modelName
}

// Close implements Engine. It releases the underlying sherpa-onnx recognizer.
func (se *SherpaEngine) Close() error {
	if se.recognizer != nil {
		se.recognizer.Delete()
		se.recognizer = nil
	}
	return nil
}

// Compile-time interface check.
var _ Engine = (*SherpaEngine)(nil)
