package format

import (
	"encoding/json"

	"github.com/arbaz/thunderstt/internal/engine"
)

// verboseResponse mirrors the OpenAI verbose_json transcription format.
type verboseResponse struct {
	Task     string            `json:"task"`
	Language string            `json:"language"`
	Duration float64           `json:"duration"`
	Text     string            `json:"text"`
	Segments []verboseSegment  `json:"segments"`
	Words    []verboseWord     `json:"words,omitempty"`
}

type verboseSegment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type verboseWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// FormatVerboseJSON_Response serialises the result as a verbose JSON object
// compatible with the OpenAI transcription API's verbose_json format.
func FormatVerboseJSON_Response(result *engine.Result) ([]byte, error) {
	resp := verboseResponse{
		Task:     "transcribe",
		Language: result.Language,
		Duration: result.Duration,
		Text:     result.FullText(),
		Segments: make([]verboseSegment, 0, len(result.Segments)),
	}

	var allWords []verboseWord

	for _, seg := range result.Segments {
		vs := verboseSegment{
			ID:           seg.ID,
			Seek:         0,
			Start:        seg.Start,
			End:          seg.End,
			Text:         seg.Text,
			Tokens:       []int{}, // token IDs not available from engine
			Temperature:  0.0,
			AvgLogprob:   seg.AvgLogProb,
			NoSpeechProb: seg.NoSpeechProb,
		}
		resp.Segments = append(resp.Segments, vs)

		for _, w := range seg.Words {
			allWords = append(allWords, verboseWord{
				Word:  w.Word,
				Start: w.Start,
				End:   w.End,
			})
		}
	}

	if len(allWords) > 0 {
		resp.Words = allWords
	}

	return json.Marshal(resp)
}
