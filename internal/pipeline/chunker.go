package pipeline

// Chunk represents a slice of audio destined for a single engine inference
// call. Start and End are in seconds relative to the original audio.
type Chunk struct {
	Start float64
	End   float64
}

// ChunkSpeechSegments groups consecutive SpeechSegments into Chunks whose
// total duration does not exceed maxDuration seconds. It never splits a
// single SpeechSegment -- cuts are always placed at silence boundaries
// between segments.
//
// Algorithm:
//  1. Walk the sorted segment list.
//  2. Accumulate segments into the current chunk.
//  3. When adding the next segment would exceed maxDuration, close the
//     current chunk and start a new one.
//  4. A single segment longer than maxDuration gets its own chunk (we do
//     not cut mid-speech).
func ChunkSpeechSegments(segments []SpeechSegment, maxDuration float64) []Chunk {
	if len(segments) == 0 {
		return nil
	}
	if maxDuration <= 0 {
		maxDuration = DefaultChunkDuration
	}

	var chunks []Chunk

	chunkStart := segments[0].Start
	chunkEnd := segments[0].End

	for i := 1; i < len(segments); i++ {
		seg := segments[i]
		proposedEnd := seg.End
		proposedDuration := proposedEnd - chunkStart

		if proposedDuration > maxDuration {
			// Close the current chunk at the end of the previous segment.
			chunks = append(chunks, Chunk{
				Start: chunkStart,
				End:   chunkEnd,
			})
			// Start a new chunk from this segment.
			chunkStart = seg.Start
			chunkEnd = seg.End
		} else {
			// Extend current chunk to include this segment.
			chunkEnd = seg.End
		}
	}

	// Emit the final chunk.
	chunks = append(chunks, Chunk{
		Start: chunkStart,
		End:   chunkEnd,
	})

	return chunks
}
