// Package engine defines the core transcription engine interface and types
// used throughout thunderstt. All engine implementations must satisfy the
// Engine interface, and all transcription results are returned as a *Result.
package engine

import "fmt"

// Engine is the central abstraction for speech-to-text backends.
// Implementations wrap concrete inference runtimes (sherpa-onnx, whisper.cpp,
// etc.) and expose a uniform Transcribe method.
type Engine interface {
	// Transcribe processes raw PCM audio samples and returns a structured
	// transcription result. The audio slice must contain float32 samples
	// normalised to [-1, 1]. sampleRate is in Hz (e.g. 16000).
	Transcribe(audio []float32, sampleRate int, opts Options) (*Result, error)

	// SupportedLanguages returns the BCP-47 language codes the engine can
	// handle. An empty slice means the engine accepts any language (auto-detect).
	SupportedLanguages() []string

	// ModelName returns a human-readable identifier for the loaded model.
	ModelName() string

	// Close releases all resources held by the engine (GPU memory, file
	// handles, etc.). The engine must not be used after Close returns.
	Close() error
}

// Options controls transcription behaviour on a per-request basis.
type Options struct {
	// Language is a BCP-47 hint (e.g. "en", "de"). Empty means auto-detect.
	Language string

	// WordTimestamps requests per-word timing information in the result.
	WordTimestamps bool

	// VADFilter enables voice-activity detection so that silent regions are
	// skipped, reducing hallucinations on quiet audio.
	VADFilter bool

	// InitialPrompt is a text prefix fed to the decoder to bias style,
	// spelling or vocabulary (supported by Whisper-family models).
	InitialPrompt string

	// MaxSegmentLength caps the character length of any single segment.
	// Zero means unlimited.
	MaxSegmentLength int
}

// Result holds the complete output of a transcription run.
type Result struct {
	// Language is the detected or forced BCP-47 language code.
	Language string

	// LanguageProb is the model's confidence [0, 1] in the detected language.
	LanguageProb float64

	// Duration is the total audio duration in seconds.
	Duration float64

	// Segments contains the time-aligned transcript pieces.
	Segments []Segment
}

// FullText concatenates all segment texts separated by spaces.
func (r *Result) FullText() string {
	if r == nil || len(r.Segments) == 0 {
		return ""
	}
	var total int
	for i := range r.Segments {
		total += len(r.Segments[i].Text) + 1
	}
	buf := make([]byte, 0, total)
	for i := range r.Segments {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, r.Segments[i].Text...)
	}
	return string(buf)
}

// IsEmpty returns true when the result contains no transcribed text.
func (r *Result) IsEmpty() bool {
	if r == nil {
		return true
	}
	for i := range r.Segments {
		if len(r.Segments[i].Text) > 0 {
			return false
		}
	}
	return true
}

// Segment represents a contiguous chunk of transcribed speech.
type Segment struct {
	// ID is the zero-based index of this segment within the result.
	ID int

	// Start is the segment start time in seconds relative to audio start.
	Start float64

	// End is the segment end time in seconds relative to audio start.
	End float64

	// Text is the transcribed text for this segment.
	Text string

	// Words holds per-word timestamps when Options.WordTimestamps is set.
	Words []Word

	// AvgLogProb is the average log-probability across tokens in the segment.
	// Lower (more negative) values indicate less confident predictions.
	AvgLogProb float64

	// NoSpeechProb is the probability that the segment contains no speech.
	NoSpeechProb float64
}

// Word represents a single word with timing and confidence information.
type Word struct {
	// Word is the text content of the word.
	Word string

	// Start is the word start time in seconds.
	Start float64

	// End is the word end time in seconds.
	End float64

	// Prob is the model's confidence [0, 1] for this word.
	Prob float64
}

// ErrEngineNotFound is returned when a requested engine name has no
// registered constructor.
type ErrEngineNotFound struct {
	Name string
}

func (e *ErrEngineNotFound) Error() string {
	return fmt.Sprintf("engine: no engine registered with name %q", e.Name)
}

// ErrEmptyAudio is returned when an empty audio buffer is passed to Transcribe.
var ErrEmptyAudio = fmt.Errorf("engine: audio buffer is empty")

// ErrUnsupportedLanguage is returned when the requested language is not
// in the engine's supported set.
type ErrUnsupportedLanguage struct {
	Language string
	Engine   string
}

func (e *ErrUnsupportedLanguage) Error() string {
	return fmt.Sprintf("engine: language %q is not supported by engine %q", e.Language, e.Engine)
}
