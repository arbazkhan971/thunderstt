// Package model provides model definitions, downloading, and cache management
// for ThunderSTT. It maintains a registry of known speech-to-text models with
// their HuggingFace source information, handles downloading model files, and
// manages the local model cache.
package model

import (
	"fmt"
	"sort"
	"sync"
)

// ModelInfo describes a known speech-to-text model and where to obtain it.
type ModelInfo struct {
	// ID is the unique identifier used to reference this model (e.g. "parakeet-tdt-0.6b-v3").
	ID string

	// Name is the human-readable display name.
	Name string

	// Engine is the inference backend required: "sherpa" or "whisper.cpp".
	Engine string

	// ModelType categorises the architecture: "parakeet" or "whisper".
	ModelType string

	// Size is a human-readable approximation of total download size.
	Size string

	// Languages lists BCP-47 language codes the model supports.
	// An empty slice means the model supports auto-detection / many languages.
	Languages []string

	// HuggingFace contains the source repository and file list for downloading.
	HuggingFace HFSource

	// OwnedBy identifies the model publisher or organisation.
	OwnedBy string

	// Files lists the filenames that must be present in the model directory
	// for the model to be considered fully cached and ready to use.
	Files []string
}

// HFSource describes where to download model files from HuggingFace Hub.
type HFSource struct {
	// Repo is the HuggingFace repository path (e.g. "csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3").
	Repo string

	// Files lists the files to download from the repository.
	Files []string
}

// europeanLanguages is the set of European languages supported by Parakeet V3 models.
var europeanLanguages = []string{
	"en", "es", "fr", "de", "it", "pt", "nl", "pl", "uk", "ro",
	"hu", "el", "sv", "cs", "bg", "sk", "hr", "da", "fi", "lt",
	"sl", "lv", "et", "mt", "ru",
}

// whisperAllLanguages is a broad set of languages supported by Whisper large models.
var whisperAllLanguages = []string{
	"en", "zh", "de", "es", "ru", "ko", "fr", "ja", "pt", "tr",
	"pl", "ca", "nl", "ar", "sv", "it", "id", "hi", "fi", "vi",
	"he", "uk", "el", "ms", "cs", "ro", "da", "hu", "ta", "no",
	"th", "ur", "hr", "bg", "lt", "la", "mi", "ml", "cy", "sk",
	"te", "fa", "lv", "bn", "sr", "az", "sl", "kn", "et", "mk",
	"br", "eu", "is", "hy", "ne", "mn", "bs", "kk", "sq", "sw",
	"gl", "mr", "pa", "si", "km", "sn", "yo", "so", "af", "oc",
	"ka", "be", "tg", "sd", "gu", "am", "yi", "lo", "uz", "fo",
	"ht", "ps", "tk", "nn", "mt", "sa", "lb", "my", "bo", "tl",
	"mg", "as", "tt", "haw", "ln", "ha", "ba", "jw", "su",
}

// registry is the internal store of known models, keyed by model ID.
var (
	registryMu sync.RWMutex
	registry   = map[string]ModelInfo{}
)

func init() {
	// -----------------------------------------------------------------------
	// Parakeet TDT 0.6B V3
	// -----------------------------------------------------------------------
	register(ModelInfo{
		ID:        "parakeet-tdt-0.6b-v3",
		Name:      "Parakeet TDT 0.6B V3",
		Engine:    "sherpa",
		ModelType: "parakeet",
		Size:      "~700 MB",
		Languages: europeanLanguages,
		HuggingFace: HFSource{
			Repo: "csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3",
			Files: []string{
				"encoder.onnx",
				"decoder.onnx",
				"tokens.txt",
			},
		},
		OwnedBy: "NVIDIA NeMo",
		Files: []string{
			"encoder.onnx",
			"decoder.onnx",
			"tokens.txt",
		},
	})

	// -----------------------------------------------------------------------
	// Parakeet TDT 0.6B V2
	// -----------------------------------------------------------------------
	register(ModelInfo{
		ID:        "parakeet-tdt-0.6b-v2",
		Name:      "Parakeet TDT 0.6B V2",
		Engine:    "sherpa",
		ModelType: "parakeet",
		Size:      "~700 MB",
		Languages: []string{"en"},
		HuggingFace: HFSource{
			Repo: "csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v2",
			Files: []string{
				"encoder.onnx",
				"decoder.onnx",
				"tokens.txt",
			},
		},
		OwnedBy: "NVIDIA NeMo",
		Files: []string{
			"encoder.onnx",
			"decoder.onnx",
			"tokens.txt",
		},
	})

	// -----------------------------------------------------------------------
	// Whisper Large V3 Turbo
	// -----------------------------------------------------------------------
	register(ModelInfo{
		ID:        "whisper-large-v3-turbo",
		Name:      "Whisper Large V3 Turbo",
		Engine:    "sherpa",
		ModelType: "whisper",
		Size:      "~1.6 GB",
		Languages: whisperAllLanguages,
		HuggingFace: HFSource{
			Repo: "csukuangfj/sherpa-onnx-whisper-large-v3-turbo",
			Files: []string{
				"large-v3-turbo-encoder.onnx",
				"large-v3-turbo-decoder.onnx",
				"large-v3-turbo-tokens.txt",
			},
		},
		OwnedBy: "OpenAI",
		Files: []string{
			"large-v3-turbo-encoder.onnx",
			"large-v3-turbo-decoder.onnx",
			"large-v3-turbo-tokens.txt",
		},
	})

	// -----------------------------------------------------------------------
	// Silero VAD
	// -----------------------------------------------------------------------
	register(ModelInfo{
		ID:        "silero-vad",
		Name:      "Silero VAD",
		Engine:    "sherpa",
		ModelType: "vad",
		Size:      "~2 MB",
		Languages: nil, // language-agnostic
		HuggingFace: HFSource{
			Repo: "csukuangfj/sherpa-onnx-silero-vad",
			Files: []string{
				"silero_vad.onnx",
			},
		},
		OwnedBy: "Silero",
		Files: []string{
			"silero_vad.onnx",
		},
	})
}

// register adds a model to the internal registry. It panics on duplicate IDs
// to catch programming errors at init time.
func register(m ModelInfo) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[m.ID]; exists {
		panic(fmt.Sprintf("model: duplicate model ID %q", m.ID))
	}
	registry[m.ID] = m
}

// GetModel returns the ModelInfo for the given model ID, or an error if the
// model is not found in the registry.
func GetModel(id string) (*ModelInfo, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	m, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("model: unknown model %q; use ListModels() to see available models", id)
	}
	return &m, nil
}

// ListModels returns all registered models sorted by ID.
func ListModels() []ModelInfo {
	registryMu.RLock()
	defer registryMu.RUnlock()

	models := make([]ModelInfo, 0, len(registry))
	for _, m := range registry {
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models
}
