package engine

// parakeetLanguages lists the BCP-47 codes supported by Parakeet NeMo models.
// Parakeet is English-only.
var parakeetLanguages = []string{"en"}

// whisperLanguages lists the BCP-47 codes supported by Whisper large models.
var whisperLanguages = []string{
	"af", "am", "ar", "as", "az", "ba", "be", "bg", "bn", "bo",
	"br", "bs", "ca", "cs", "cy", "da", "de", "el", "en", "es",
	"et", "eu", "fa", "fi", "fo", "fr", "gl", "gu", "ha", "haw",
	"he", "hi", "hr", "ht", "hu", "hy", "id", "is", "it", "ja",
	"jw", "ka", "kk", "km", "kn", "ko", "la", "lb", "ln", "lo",
	"lt", "lv", "mg", "mi", "mk", "ml", "mn", "mr", "ms", "mt",
	"my", "ne", "nl", "nn", "no", "oc", "pa", "pl", "ps", "pt",
	"ro", "ru", "sa", "sd", "si", "sk", "sl", "sn", "so", "sq",
	"sr", "su", "sv", "sw", "ta", "te", "tg", "th", "tk", "tl",
	"tr", "tt", "uk", "ur", "uz", "vi", "yi", "yo", "zh",
}

// ModelInfo describes a well-known model for discovery purposes.
type ModelInfo struct {
	// Name is the registry key (e.g. "parakeet-tdt-0.6b-v3").
	Name string

	// Type is the model family: "parakeet" or "whisper".
	Type string

	// Languages lists supported BCP-47 codes.
	Languages []string

	// Description is a short human-readable summary.
	Description string
}

// KnownModels returns metadata for every pre-registered model. This is
// intended for CLI help text and API discovery endpoints.
func KnownModels() []ModelInfo {
	return []ModelInfo{
		{
			Name:        "parakeet-tdt-0.6b-v3",
			Type:        "parakeet",
			Languages:   []string{"en"},
			Description: "NVIDIA Parakeet TDT 0.6B v3 — fast English-only NeMo transducer",
		},
		{
			Name:        "parakeet-tdt-0.6b-v2",
			Type:        "parakeet",
			Languages:   []string{"en"},
			Description: "NVIDIA Parakeet TDT 0.6B v2 — English-only NeMo transducer",
		},
		{
			Name:        "whisper-large-v3-turbo",
			Type:        "whisper",
			Languages:   whisperLanguages,
			Description: "OpenAI Whisper Large v3 Turbo — fast multilingual",
		},
		{
			Name:        "whisper-large-v3",
			Type:        "whisper",
			Languages:   whisperLanguages,
			Description: "OpenAI Whisper Large v3 — highest-accuracy multilingual",
		},
		{
			Name:        "whisper-medium",
			Type:        "whisper",
			Languages:   whisperLanguages,
			Description: "OpenAI Whisper Medium — balanced speed/accuracy multilingual",
		},
	}
}
