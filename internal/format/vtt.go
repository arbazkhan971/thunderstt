package format

import (
	"fmt"
	"strings"

	"github.com/arbaz/thunderstt/internal/engine"
)

// FormatVTTResponse converts the transcription result into WebVTT format.
//
// Example output:
//
//	WEBVTT
//
//	00:00:00.000 --> 00:00:04.200
//	Hello everyone, welcome to the meeting.
//
//	00:00:04.500 --> 00:00:08.100
//	Let's get started.
func FormatVTTResponse(result *engine.Result) []byte {
	var b strings.Builder

	b.WriteString("WEBVTT\n")

	for _, seg := range result.Segments {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s --> %s\n", formatVTTTime(seg.Start), formatVTTTime(seg.End))
		b.WriteString(strings.TrimSpace(seg.Text))
		b.WriteByte('\n')
	}

	return []byte(b.String())
}

// formatVTTTime converts seconds to the WebVTT timecode format HH:MM:SS.mmm.
func formatVTTTime(seconds float64) string {
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
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
