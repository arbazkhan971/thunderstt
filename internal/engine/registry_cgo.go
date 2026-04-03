//go:build cgo

package engine

// init registers the well-known sherpa-onnx backed models. This file is only
// compiled when CGO is enabled because SherpaEngine depends on the native
// sherpa-onnx C library via cgo.
func init() {
	// --- Parakeet NeMo TDT models (English-only) ---

	RegisterEngine("parakeet-tdt-0.6b-v3", func(modelPath string) (Engine, error) {
		return NewSherpaEngine(modelPath, "parakeet",
			WithModelName("parakeet-tdt-0.6b-v3"),
		)
	})

	RegisterEngine("parakeet-tdt-0.6b-v2", func(modelPath string) (Engine, error) {
		return NewSherpaEngine(modelPath, "parakeet",
			WithModelName("parakeet-tdt-0.6b-v2"),
		)
	})

	// --- Whisper models (multilingual) ---

	RegisterEngine("whisper-large-v3-turbo", func(modelPath string) (Engine, error) {
		return NewSherpaEngine(modelPath, "whisper",
			WithModelName("whisper-large-v3-turbo"),
		)
	})

	RegisterEngine("whisper-large-v3", func(modelPath string) (Engine, error) {
		return NewSherpaEngine(modelPath, "whisper",
			WithModelName("whisper-large-v3"),
		)
	})

	RegisterEngine("whisper-medium", func(modelPath string) (Engine, error) {
		return NewSherpaEngine(modelPath, "whisper",
			WithModelName("whisper-medium"),
		)
	})
}
