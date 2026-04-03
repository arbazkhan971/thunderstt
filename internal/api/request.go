package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/arbaz/thunderstt/internal/format"
)

// TranscribeRequest holds the parsed and validated fields from a multipart
// transcription request.
type TranscribeRequest struct {
	// File is the uploaded audio file handle.
	File multipart.File

	// FileHeader contains the filename and size metadata.
	FileHeader *multipart.FileHeader

	// Model is the requested model name (e.g. "whisper-large-v3-turbo").
	Model string

	// Language is an optional BCP-47 language hint (e.g. "en").
	Language string

	// ResponseFormat controls the output serialisation.
	// One of: json, verbose_json, text, srt, vtt.
	ResponseFormat string

	// TimestampGranularities specifies the granularity levels requested.
	// Valid values are "word" and "segment".
	TimestampGranularities []string
}

// knownModels lists model IDs that the server recognises. "auto" is a
// special value that lets the server choose the default model.
var knownModels = map[string]bool{
	"auto":                     true,
	"whisper-large-v3-turbo":   true,
	"whisper-large-v3":         true,
	"whisper-large-v2":         true,
	"whisper-medium":           true,
	"whisper-small":            true,
	"whisper-base":             true,
	"whisper-tiny":             true,
	"parakeet-tdt-0.6b-v3":    true,
}

// validTimestampGranularities lists acceptable values for the
// timestamp_granularities[] parameter.
var validTimestampGranularities = map[string]bool{
	"word":    true,
	"segment": true,
}

// ParseTranscribeRequest extracts and validates all fields from a multipart
// transcription request. The caller is responsible for closing the returned
// File handle.
func ParseTranscribeRequest(r *http.Request) (*TranscribeRequest, error) {
	// File field.
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("missing required field \"file\": %w", err)
	}

	// Model (required by OpenAI spec, but we default to "auto").
	model := strings.TrimSpace(r.FormValue("model"))
	if model == "" {
		model = "auto"
	}
	if !knownModels[model] {
		file.Close()
		return nil, fmt.Errorf("unknown model %q; see GET /v1/models for available models", model)
	}

	// Language (optional).
	language := strings.TrimSpace(r.FormValue("language"))

	// Response format (optional, defaults to json).
	responseFormat := strings.TrimSpace(r.FormValue("response_format"))
	if responseFormat == "" {
		responseFormat = format.FormatJSON
	}
	if !format.IsValidFormat(responseFormat) {
		file.Close()
		return nil, fmt.Errorf("invalid response_format %q; must be one of: json, verbose_json, text, srt, vtt", responseFormat)
	}

	// Timestamp granularities (optional, may appear multiple times).
	var granularities []string
	if vals, ok := r.MultipartForm.Value["timestamp_granularities[]"]; ok {
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if !validTimestampGranularities[v] {
				file.Close()
				return nil, fmt.Errorf("invalid timestamp_granularities value %q; must be \"word\" or \"segment\"", v)
			}
			granularities = append(granularities, v)
		}
	}

	return &TranscribeRequest{
		File:                   file,
		FileHeader:             header,
		Model:                  model,
		Language:               language,
		ResponseFormat:         responseFormat,
		TimestampGranularities: granularities,
	}, nil
}

// WantsWordTimestamps returns true if the request includes "word" in the
// timestamp granularities.
func (req *TranscribeRequest) WantsWordTimestamps() bool {
	for _, g := range req.TimestampGranularities {
		if g == "word" {
			return true
		}
	}
	return false
}
