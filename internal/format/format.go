// Package format converts engine.Result into various output representations
// (JSON, verbose JSON, plain text, SRT, WebVTT) as required by the OpenAI
// transcription API.
package format

import (
	"fmt"

	"github.com/arbaz/thunderstt/internal/engine"
)

// Supported response_format values.
const (
	FormatJSON        = "json"
	FormatVerboseJSON = "verbose_json"
	FormatText        = "text"
	FormatSRT         = "srt"
	FormatVTT         = "vtt"
)

// ContentTypeJSON is used for json and verbose_json formats.
const ContentTypeJSON = "application/json; charset=utf-8"

// ContentTypeText is used for text, srt, and vtt formats.
const ContentTypeText = "text/plain; charset=utf-8"

// ValidFormats enumerates all accepted response_format values.
var ValidFormats = map[string]bool{
	FormatJSON:        true,
	FormatVerboseJSON: true,
	FormatText:        true,
	FormatSRT:         true,
	FormatVTT:         true,
}

// IsValidFormat checks whether the given format string is supported.
func IsValidFormat(f string) bool {
	return ValidFormats[f]
}

// FormatResult converts a transcription result into the requested output
// format. It returns the serialised bytes, the appropriate Content-Type
// header value, and any error encountered during formatting.
func FormatResult(result *engine.Result, format string) ([]byte, string, error) {
	if result == nil {
		return nil, "", fmt.Errorf("format: result is nil")
	}

	switch format {
	case FormatJSON:
		data, err := FormatJSON_Response(result)
		return data, ContentTypeJSON, err
	case FormatVerboseJSON:
		data, err := FormatVerboseJSON_Response(result)
		return data, ContentTypeJSON, err
	case FormatText:
		data := FormatTextResponse(result)
		return data, ContentTypeText, nil
	case FormatSRT:
		data := FormatSRTResponse(result)
		return data, ContentTypeText, nil
	case FormatVTT:
		data := FormatVTTResponse(result)
		return data, ContentTypeText, nil
	default:
		return nil, "", fmt.Errorf("format: unsupported response_format %q", format)
	}
}
