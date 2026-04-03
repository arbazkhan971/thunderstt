// Package engine — NoopEngine is a placeholder Engine implementation that
// returns empty results. It is intended for development and testing when a
// real inference backend (e.g. sherpa-onnx) is unavailable because CGo is
// disabled or the required native libraries are not installed.
package engine

// NoopEngine implements the Engine interface without performing any real
// inference. Every call to Transcribe returns an empty Result with no error.
type NoopEngine struct {
	model string
}

// NewNoopEngine creates a NoopEngine tagged with the given model name.
func NewNoopEngine(modelName string) *NoopEngine {
	return &NoopEngine{model: modelName}
}

// Transcribe implements Engine. It returns an empty Result with zero segments.
func (n *NoopEngine) Transcribe(_ []float32, _ int, opts Options) (*Result, error) {
	return &Result{
		Language:     opts.Language,
		LanguageProb: 0,
		Duration:     0,
		Segments:     nil,
	}, nil
}

// SupportedLanguages implements Engine. The noop engine claims no specific
// language support (auto-detect / any).
func (n *NoopEngine) SupportedLanguages() []string {
	return nil
}

// ModelName implements Engine.
func (n *NoopEngine) ModelName() string {
	return n.model
}

// Close implements Engine. It is a no-op.
func (n *NoopEngine) Close() error {
	return nil
}

// Compile-time interface check.
var _ Engine = (*NoopEngine)(nil)
