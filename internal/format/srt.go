package format

import (
	"fmt"
	"strings"

	"github.com/arbaz/thunderstt/internal/engine"
)

// FormatSRTResponse converts the transcription result into the SubRip (SRT)
// subtitle format.
//
// Example output:
//
//	1
//	00:00:00,000 --> 00:00:04,200
//	Hello everyone, welcome to the meeting.
//
//	2
//	00:00:04,500 --> 00:00:08,100
//	Let's get started.
func FormatSRTResponse(result *engine.Result) []byte {
	var b strings.Builder

	for i, seg := range result.Segments {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Sequence number (1-based).
		fmt.Fprintf(&b, "%d\n", i+1)
		// Timecodes.
		fmt.Fprintf(&b, "%s --> %s\n", formatSRTTime(seg.Start), formatSRTTime(seg.End))
		// Text content.
		b.WriteString(strings.TrimSpace(seg.Text))
		b.WriteByte('\n')
	}

	return []byte(b.String())
}

// formatSRTTime converts seconds to the SRT timecode format HH:MM:SS,mmm.
func formatSRTTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMs := int64(seconds * 1000)
	h := totalMs / 3600000
	totalMs %= 3600000
	m := totalMs / 60000
	totalMs %= 60000
	s := totalMs / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
