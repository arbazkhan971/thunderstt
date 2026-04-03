// Package pipeline orchestrates the full speech-to-text pipeline: decode,
// resample, VAD, chunk, transcribe, and stitch.
package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/arbaz/thunderstt/internal/audio"
	"github.com/arbaz/thunderstt/internal/engine"
)

// DefaultChunkDuration is the target chunk length in seconds when splitting
// long audio for inference.
const DefaultChunkDuration = 20.0

// PipelineConfig controls pipeline behaviour.
type PipelineConfig struct {
	// VADEnabled enables Silero VAD-based speech segmentation before
	// transcription. When false, audio is chunked at fixed intervals.
	VADEnabled bool

	// ChunkDuration is the maximum duration in seconds per chunk sent to the
	// engine. Defaults to DefaultChunkDuration if <= 0.
	ChunkDuration float64

	// WordTimestamps requests word-level timing from the engine.
	WordTimestamps bool

	// Language is a BCP-47 hint passed to the engine (empty = auto-detect).
	Language string

	// VADModelPath is the filesystem path to the Silero VAD ONNX model.
	// Only required when VADEnabled is true.
	VADModelPath string
}

// Pipeline ties together audio decoding, optional VAD, chunking, engine
// inference, and result stitching.
type Pipeline struct {
	engine engine.Engine
	vad    *VAD
	config PipelineConfig
	ready  atomic.Bool
}

// New creates a Pipeline that delegates transcription to the given engine.
// This is the simple constructor that does not initialise VAD. Use
// NewPipeline for the full-featured variant.
func New(eng engine.Engine) *Pipeline {
	p := &Pipeline{
		engine: eng,
		config: PipelineConfig{
			ChunkDuration: DefaultChunkDuration,
		},
	}
	if eng != nil {
		p.ready.Store(true)
	}
	return p
}

// NewPipeline creates a fully configured Pipeline. If cfg.VADEnabled is true
// and cfg.VADModelPath is non-empty, a Silero VAD instance is initialised.
// When VAD initialisation fails the error is returned; callers that want to
// proceed without VAD should set VADEnabled = false.
func NewPipeline(eng engine.Engine, cfg PipelineConfig) (*Pipeline, error) {
	if cfg.ChunkDuration <= 0 {
		cfg.ChunkDuration = DefaultChunkDuration
	}

	p := &Pipeline{
		engine: eng,
		config: cfg,
	}

	if cfg.VADEnabled && cfg.VADModelPath != "" {
		v, err := NewVAD(cfg.VADModelPath)
		if err != nil {
			return nil, fmt.Errorf("pipeline: init VAD: %w", err)
		}
		p.vad = v
	}

	if eng != nil {
		p.ready.Store(true)
	}

	return p, nil
}

// Ready reports whether the pipeline has been fully initialised and can
// accept transcription requests.
func (p *Pipeline) Ready() bool {
	return p.ready.Load()
}

// SetReady allows external callers (e.g. model loaders) to flip the
// readiness flag once all resources are available.
func (p *Pipeline) SetReady(v bool) {
	p.ready.Store(v)
}

// ModelName returns the name of the loaded model, or empty if no engine is set.
func (p *Pipeline) ModelName() string {
	if p.engine == nil {
		return ""
	}
	return p.engine.ModelName()
}

// Close releases resources held by the pipeline (VAD and engine).
func (p *Pipeline) Close() error {
	p.ready.Store(false)
	if p.vad != nil {
		p.vad.Close()
	}
	if p.engine != nil {
		return p.engine.Close()
	}
	return nil
}

// TranscribeFile decodes the audio file at path and runs the full pipeline.
// This is the primary entry point used by the HTTP handler.
func (p *Pipeline) TranscribeFile(ctx context.Context, path string, opts engine.Options) (*engine.Result, error) {
	if !p.ready.Load() {
		return nil, fmt.Errorf("pipeline: not ready, model is still loading")
	}

	samples, sampleRate, err := decodeAudioFile(path)
	if err != nil {
		return nil, fmt.Errorf("pipeline: decode: %w", err)
	}

	// Merge per-request options with pipeline-level config.
	merged := p.mergeOptions(opts)
	return p.processAudioInternal(ctx, samples, sampleRate, merged)
}

// Process decodes the audio file at audioPath, runs the full pipeline, and
// returns a combined transcription result.
func (p *Pipeline) Process(audioPath string) (*engine.Result, error) {
	samples, sampleRate, err := decodeAudioFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("pipeline: decode: %w", err)
	}

	opts := engine.Options{
		Language:       p.config.Language,
		WordTimestamps: p.config.WordTimestamps,
	}
	return p.processAudioInternal(context.Background(), samples, sampleRate, opts)
}

// ProcessAudio runs the pipeline on pre-decoded audio samples. The samples
// must be interleaved float32 PCM normalised to [-1, 1].
func (p *Pipeline) ProcessAudio(samples []float32, sampleRate int) (*engine.Result, error) {
	opts := engine.Options{
		Language:       p.config.Language,
		WordTimestamps: p.config.WordTimestamps,
	}
	return p.processAudioInternal(context.Background(), samples, sampleRate, opts)
}

// processAudioInternal is the core pipeline implementation.
func (p *Pipeline) processAudioInternal(ctx context.Context, samples []float32, sampleRate int, opts engine.Options) (*engine.Result, error) {
	// 1. Mix to mono and resample to 16 kHz.
	const targetRate = 16000
	mono := audio.ToMono(samples, guessChannels(samples, sampleRate))
	resampled := audio.Resample(mono, sampleRate, targetRate)

	totalDuration := float64(len(resampled)) / float64(targetRate)

	// 2. Segment speech via VAD or treat entire audio as one segment.
	segments, err := p.detectSpeechSegments(resampled, targetRate)
	if err != nil {
		return nil, fmt.Errorf("pipeline: VAD: %w", err)
	}

	// 3. Chunk segments to engine-friendly sizes.
	chunks := ChunkSpeechSegments(segments, p.config.ChunkDuration)

	// 4. Transcribe each chunk.
	chunkResults := make([]ChunkResult, 0, len(chunks))
	for _, c := range chunks {
		// Check for context cancellation between chunks.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		startSample := int(c.Start * float64(targetRate))
		endSample := int(c.End * float64(targetRate))
		if startSample < 0 {
			startSample = 0
		}
		if endSample > len(resampled) {
			endSample = len(resampled)
		}
		chunkAudio := resampled[startSample:endSample]
		if len(chunkAudio) == 0 {
			continue
		}

		res, err := p.engine.Transcribe(chunkAudio, targetRate, opts)
		if err != nil {
			return nil, fmt.Errorf("pipeline: transcribe chunk [%.2f-%.2f]: %w", c.Start, c.End, err)
		}

		chunkResults = append(chunkResults, ChunkResult{
			Offset: c.Start,
			Result: res,
		})
	}

	// 5. Stitch chunk results into a single Result.
	combined := StitchResults(chunkResults)
	combined.Duration = totalDuration

	// Clean up segment text.
	for i := range combined.Segments {
		combined.Segments[i].Text = strings.TrimSpace(combined.Segments[i].Text)
	}

	return combined, nil
}

// detectSpeechSegments returns speech segments via VAD, or a single segment
// spanning the whole audio when VAD is disabled.
func (p *Pipeline) detectSpeechSegments(samples []float32, sampleRate int) ([]SpeechSegment, error) {
	if p.vad != nil && p.config.VADEnabled {
		segs, err := p.vad.DetectSpeech(samples, sampleRate)
		if err != nil {
			return nil, err
		}
		if len(segs) == 0 {
			// VAD found no speech; still send the whole audio so the engine
			// gets a chance to transcribe.
			dur := float64(len(samples)) / float64(sampleRate)
			return []SpeechSegment{{Start: 0, End: dur}}, nil
		}
		return segs, nil
	}

	// No VAD -- treat entire audio as speech.
	dur := float64(len(samples)) / float64(sampleRate)
	return []SpeechSegment{{Start: 0, End: dur}}, nil
}

// mergeOptions creates engine.Options that combine per-request opts with the
// pipeline-level configuration defaults.
func (p *Pipeline) mergeOptions(opts engine.Options) engine.Options {
	if opts.Language == "" {
		opts.Language = p.config.Language
	}
	if !opts.WordTimestamps {
		opts.WordTimestamps = p.config.WordTimestamps
	}
	return opts
}

// decodeAudioFile attempts native Go decoders first, then falls back to
// ffmpeg for unsupported formats.
func decodeAudioFile(path string) ([]float32, int, error) {
	samples, rate, err := audio.DecodeFile(path)
	if err == nil {
		return samples, rate, nil
	}

	// Native decoder failed -- try ffmpeg.
	if audio.IsFFmpegAvailable() {
		return audio.DecodeWithFFmpeg(path)
	}

	return nil, 0, fmt.Errorf("native decode failed (%w) and ffmpeg is not available", err)
}

// guessChannels makes a best-effort guess at the channel count. Our native
// decoders return interleaved PCM but do not propagate channel metadata
// through the pipeline, so we conservatively assume mono. The caller should
// call ToMono explicitly when the true channel count is known.
func guessChannels(_ []float32, _ int) int {
	return 1
}
