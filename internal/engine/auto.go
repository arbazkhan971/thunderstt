package engine

import (
	"fmt"
	"strings"
	"sync"
)

// AutoEngine holds a primary engine (typically Parakeet for English) and a
// fallback engine (typically Whisper for multilingual). It routes each
// Transcribe call to the best-suited backend based on the language hint,
// falling back transparently when the primary engine fails or returns empty
// output.
type AutoEngine struct {
	primary  Engine
	fallback Engine

	// supportedOnce lazily computes the union of both engines' languages.
	supportedOnce sync.Once
	supported     []string
}

// NewAutoEngine constructs an AutoEngine. primary is used first for
// languages it supports; fallback covers all remaining languages.
// Both engines must be non-nil.
func NewAutoEngine(primary, fallback Engine) (*AutoEngine, error) {
	if primary == nil {
		return nil, fmt.Errorf("auto: primary engine must not be nil")
	}
	if fallback == nil {
		return nil, fmt.Errorf("auto: fallback engine must not be nil")
	}
	return &AutoEngine{
		primary:  primary,
		fallback: fallback,
	}, nil
}

// Transcribe implements Engine.
//
// Routing logic:
//  1. If a language hint is provided and the primary engine supports it, use
//     the primary engine.
//  2. If a language hint is provided and only the fallback supports it, use
//     the fallback.
//  3. If no language hint is given, try the primary engine first. If it
//     fails or returns empty text, retry with the fallback.
func (a *AutoEngine) Transcribe(audio []float32, sampleRate int, opts Options) (*Result, error) {
	if len(audio) == 0 {
		return nil, ErrEmptyAudio
	}

	lang := strings.ToLower(opts.Language)

	// --- Explicit language hint ---
	if lang != "" {
		if languageIn(lang, a.primary.SupportedLanguages()) {
			return a.primary.Transcribe(audio, sampleRate, opts)
		}
		if languageIn(lang, a.fallback.SupportedLanguages()) {
			return a.fallback.Transcribe(audio, sampleRate, opts)
		}
		return nil, &ErrUnsupportedLanguage{
			Language: lang,
			Engine:   a.ModelName(),
		}
	}

	// --- No language hint: try primary, then fallback ---
	result, err := a.primary.Transcribe(audio, sampleRate, opts)
	if err == nil && !result.IsEmpty() {
		return result, nil
	}

	// Primary failed or returned empty — try fallback.
	fallbackResult, fallbackErr := a.fallback.Transcribe(audio, sampleRate, opts)
	if fallbackErr != nil {
		// Both engines failed. Return the primary error if available,
		// otherwise the fallback error, so the caller sees the most
		// relevant diagnostic.
		if err != nil {
			return nil, fmt.Errorf("auto: primary failed: %w; fallback also failed: %v", err, fallbackErr)
		}
		return nil, fmt.Errorf("auto: fallback failed: %w", fallbackErr)
	}

	return fallbackResult, nil
}

// SupportedLanguages implements Engine. It returns the deduplicated union of
// both engines' language lists.
func (a *AutoEngine) SupportedLanguages() []string {
	a.supportedOnce.Do(func() {
		a.supported = unionLanguages(
			a.primary.SupportedLanguages(),
			a.fallback.SupportedLanguages(),
		)
	})
	// Return a copy to prevent callers from mutating the cached slice.
	dst := make([]string, len(a.supported))
	copy(dst, a.supported)
	return dst
}

// ModelName implements Engine.
func (a *AutoEngine) ModelName() string {
	return fmt.Sprintf("auto(%s+%s)", a.primary.ModelName(), a.fallback.ModelName())
}

// Close implements Engine. It closes both the primary and fallback engines,
// collecting any errors.
func (a *AutoEngine) Close() error {
	var errs []string

	if err := a.primary.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("primary: %v", err))
	}
	if err := a.fallback.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("fallback: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("auto: close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// languageIn checks whether lang is present in the list.
func languageIn(lang string, languages []string) bool {
	for _, l := range languages {
		if strings.EqualFold(l, lang) {
			return true
		}
	}
	return false
}

// unionLanguages returns a deduplicated, order-preserving union of a and b.
func unionLanguages(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))

	for _, l := range a {
		lower := strings.ToLower(l)
		if _, ok := seen[lower]; !ok {
			seen[lower] = struct{}{}
			result = append(result, lower)
		}
	}
	for _, l := range b {
		lower := strings.ToLower(l)
		if _, ok := seen[lower]; !ok {
			seen[lower] = struct{}{}
			result = append(result, lower)
		}
	}

	return result
}

// Compile-time interface check.
var _ Engine = (*AutoEngine)(nil)
