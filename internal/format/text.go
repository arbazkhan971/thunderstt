package format

import (
	"github.com/arbaz/thunderstt/internal/engine"
)

// FormatTextResponse returns the full transcription as plain UTF-8 text.
func FormatTextResponse(result *engine.Result) []byte {
	return []byte(result.FullText())
}
