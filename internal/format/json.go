package format

import (
	"encoding/json"

	"github.com/arbaz/thunderstt/internal/engine"
)

// jsonResponse is the minimal OpenAI-compatible transcription response.
type jsonResponse struct {
	Text string `json:"text"`
}

// FormatJSON_Response serialises the result as a minimal JSON object
// containing only the full transcription text.
//
//	{"text": "Hello world"}
func FormatJSON_Response(result *engine.Result) ([]byte, error) {
	resp := jsonResponse{
		Text: result.FullText(),
	}
	return json.Marshal(resp)
}
