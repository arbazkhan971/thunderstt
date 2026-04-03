package pipeline

import (
	"github.com/arbaz/thunderstt/internal/engine"
)

// ChunkResult pairs an engine transcription result with the time offset (in
// seconds) of the chunk relative to the original audio.
type ChunkResult struct {
	Offset float64        // Start time of this chunk in the original audio.
	Result *engine.Result // Transcription result for the chunk.
}

// StitchResults merges multiple ChunkResults into a single engine.Result.
// For each chunk:
//   - Segment and word timestamps are shifted forward by Offset.
//   - Segment IDs are re-numbered sequentially starting from 0.
//   - The combined Duration is set to the maximum end time across all
//     segments (callers may override this with the true audio duration).
//   - Language and LanguageProb are taken from the first non-empty result.
func StitchResults(chunks []ChunkResult) *engine.Result {
	combined := &engine.Result{}

	if len(chunks) == 0 {
		return combined
	}

	segID := 0
	var maxEnd float64

	for _, cr := range chunks {
		if cr.Result == nil {
			continue
		}

		// Inherit language info from the first chunk that has it.
		if combined.Language == "" && cr.Result.Language != "" {
			combined.Language = cr.Result.Language
			combined.LanguageProb = cr.Result.LanguageProb
		}

		for _, seg := range cr.Result.Segments {
			shifted := engine.Segment{
				ID:           segID,
				Start:        seg.Start + cr.Offset,
				End:          seg.End + cr.Offset,
				Text:         seg.Text,
				AvgLogProb:   seg.AvgLogProb,
				NoSpeechProb: seg.NoSpeechProb,
				Words:        make([]engine.Word, len(seg.Words)),
			}

			for j, w := range seg.Words {
				shifted.Words[j] = engine.Word{
					Word:  w.Word,
					Start: w.Start + cr.Offset,
					End:   w.End + cr.Offset,
					Prob:  w.Prob,
				}
			}

			if shifted.End > maxEnd {
				maxEnd = shifted.End
			}

			combined.Segments = append(combined.Segments, shifted)
			segID++
		}
	}

	combined.Duration = maxEnd

	return combined
}
