//go:build !cgo

package format

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arbaz/thunderstt/internal/engine"
)

// sampleResult returns a representative engine.Result for testing formatters.
func sampleResult() *engine.Result {
	return &engine.Result{
		Language:     "en",
		LanguageProb: 0.95,
		Duration:     10.5,
		Segments: []engine.Segment{
			{
				ID:           0,
				Start:        0.0,
				End:          4.2,
				Text:         "Hello everyone",
				AvgLogProb:   -0.3,
				NoSpeechProb: 0.01,
				Words: []engine.Word{
					{Word: "Hello", Start: 0.0, End: 1.5, Prob: 0.99},
					{Word: "everyone", Start: 1.6, End: 4.2, Prob: 0.97},
				},
			},
			{
				ID:           1,
				Start:        4.5,
				End:          8.1,
				Text:         "welcome to the meeting",
				AvgLogProb:   -0.25,
				NoSpeechProb: 0.02,
				Words: []engine.Word{
					{Word: "welcome", Start: 4.5, End: 5.5, Prob: 0.98},
					{Word: "to", Start: 5.6, End: 5.9, Prob: 0.99},
					{Word: "the", Start: 6.0, End: 6.3, Prob: 0.99},
					{Word: "meeting", Start: 6.4, End: 8.1, Prob: 0.96},
				},
			},
		},
	}
}

func TestFormatResult_JSON(t *testing.T) {
	result := sampleResult()
	data, contentType, err := FormatResult(result, FormatJSON)
	if err != nil {
		t.Fatalf("FormatResult json failed: %v", err)
	}
	if contentType != ContentTypeJSON {
		t.Errorf("expected content type %q, got %q", ContentTypeJSON, contentType)
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is invalid: %v\nraw: %s", err, data)
	}

	// Check that "text" field is present and matches FullText.
	text, ok := parsed["text"].(string)
	if !ok {
		t.Fatal("JSON output missing 'text' field")
	}
	expected := result.FullText()
	if text != expected {
		t.Errorf("JSON text = %q, want %q", text, expected)
	}
}

func TestFormatResult_VerboseJSON(t *testing.T) {
	result := sampleResult()
	data, contentType, err := FormatResult(result, FormatVerboseJSON)
	if err != nil {
		t.Fatalf("FormatResult verbose_json failed: %v", err)
	}
	if contentType != ContentTypeJSON {
		t.Errorf("expected content type %q, got %q", ContentTypeJSON, contentType)
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("verbose JSON output is invalid: %v\nraw: %s", err, data)
	}

	// Verify expected fields.
	if parsed["task"] != "transcribe" {
		t.Errorf("expected task=transcribe, got %v", parsed["task"])
	}
	if parsed["language"] != "en" {
		t.Errorf("expected language=en, got %v", parsed["language"])
	}
	if _, ok := parsed["duration"]; !ok {
		t.Error("verbose JSON missing 'duration' field")
	}
	if _, ok := parsed["text"]; !ok {
		t.Error("verbose JSON missing 'text' field")
	}

	// Verify segments array.
	segments, ok := parsed["segments"].([]interface{})
	if !ok {
		t.Fatal("verbose JSON missing 'segments' array")
	}
	if len(segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(segments))
	}

	// Verify words array is present (our sample has words).
	words, ok := parsed["words"].([]interface{})
	if !ok {
		t.Fatal("verbose JSON missing 'words' array")
	}
	if len(words) != 6 {
		t.Errorf("expected 6 words, got %d", len(words))
	}
}

func TestFormatResult_Text(t *testing.T) {
	result := sampleResult()
	data, contentType, err := FormatResult(result, FormatText)
	if err != nil {
		t.Fatalf("FormatResult text failed: %v", err)
	}
	if contentType != ContentTypeText {
		t.Errorf("expected content type %q, got %q", ContentTypeText, contentType)
	}

	got := string(data)
	expected := result.FullText()
	if got != expected {
		t.Errorf("text output = %q, want %q", got, expected)
	}
}

func TestFormatResult_SRT(t *testing.T) {
	result := sampleResult()
	data, contentType, err := FormatResult(result, FormatSRT)
	if err != nil {
		t.Fatalf("FormatResult srt failed: %v", err)
	}
	if contentType != ContentTypeText {
		t.Errorf("expected content type %q, got %q", ContentTypeText, contentType)
	}

	srt := string(data)

	// SRT must contain sequence numbers.
	if !strings.Contains(srt, "1\n") {
		t.Error("SRT missing sequence number 1")
	}
	if !strings.Contains(srt, "2\n") {
		t.Error("SRT missing sequence number 2")
	}

	// Verify SRT timecode format: HH:MM:SS,mmm
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:04,200") {
		t.Errorf("SRT missing expected timecode for segment 1.\nGot:\n%s", srt)
	}
	if !strings.Contains(srt, "00:00:04,500 --> 00:00:08,100") {
		t.Errorf("SRT missing expected timecode for segment 2.\nGot:\n%s", srt)
	}

	// Verify SRT uses comma in timecodes (not period).
	if strings.Contains(srt, "00:00:00.000") {
		t.Error("SRT should use comma in timecodes, not period")
	}

	// Verify segment text is present.
	if !strings.Contains(srt, "Hello everyone") {
		t.Error("SRT missing segment text 'Hello everyone'")
	}
	if !strings.Contains(srt, "welcome to the meeting") {
		t.Error("SRT missing segment text 'welcome to the meeting'")
	}
}

func TestFormatResult_VTT(t *testing.T) {
	result := sampleResult()
	data, contentType, err := FormatResult(result, FormatVTT)
	if err != nil {
		t.Fatalf("FormatResult vtt failed: %v", err)
	}
	if contentType != ContentTypeText {
		t.Errorf("expected content type %q, got %q", ContentTypeText, contentType)
	}

	vtt := string(data)

	// WebVTT must start with "WEBVTT".
	if !strings.HasPrefix(vtt, "WEBVTT") {
		t.Errorf("VTT output must start with 'WEBVTT', got: %q", vtt[:min(20, len(vtt))])
	}

	// VTT uses period in timecodes (not comma like SRT).
	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:04.200") {
		t.Errorf("VTT missing expected timecode for segment 1.\nGot:\n%s", vtt)
	}
	if !strings.Contains(vtt, "00:00:04.500 --> 00:00:08.100") {
		t.Errorf("VTT missing expected timecode for segment 2.\nGot:\n%s", vtt)
	}

	// Verify segment text is present.
	if !strings.Contains(vtt, "Hello everyone") {
		t.Error("VTT missing segment text 'Hello everyone'")
	}
}

func TestFormatResult_NilResult(t *testing.T) {
	_, _, err := FormatResult(nil, FormatJSON)
	if err == nil {
		t.Error("expected error for nil result, got nil")
	}
}

func TestFormatResult_UnsupportedFormat(t *testing.T) {
	result := sampleResult()
	_, _, err := FormatResult(result, "xml")
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
}

func TestIsValidFormat(t *testing.T) {
	valids := []string{"json", "verbose_json", "text", "srt", "vtt"}
	for _, f := range valids {
		if !IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = false, want true", f)
		}
	}

	invalids := []string{"xml", "csv", "", "JSON"}
	for _, f := range invalids {
		if IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = true, want false", f)
		}
	}
}

func TestFormatResult_EmptySegments(t *testing.T) {
	result := &engine.Result{
		Language: "en",
		Duration: 0.0,
	}

	// JSON should still work with empty segments.
	data, _, err := FormatResult(result, FormatJSON)
	if err != nil {
		t.Fatalf("FormatResult json with empty segments failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON for empty result: %v", err)
	}
	if parsed["text"] != "" {
		t.Errorf("expected empty text, got %q", parsed["text"])
	}

	// SRT should produce empty output.
	srtData, _, err := FormatResult(result, FormatSRT)
	if err != nil {
		t.Fatalf("FormatResult srt with empty segments failed: %v", err)
	}
	if len(srtData) != 0 {
		t.Errorf("expected empty SRT output, got %q", string(srtData))
	}

	// VTT should still have the WEBVTT header.
	vttData, _, err := FormatResult(result, FormatVTT)
	if err != nil {
		t.Fatalf("FormatResult vtt with empty segments failed: %v", err)
	}
	if !strings.HasPrefix(string(vttData), "WEBVTT") {
		t.Error("VTT with empty segments should still start with WEBVTT")
	}
}

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

// TestFormatResult_nilResult verifies that nil result does not panic for every
// supported format and always returns an error.
func TestFormatResult_nilResult(t *testing.T) {
	formats := []string{FormatJSON, FormatVerboseJSON, FormatText, FormatSRT, FormatVTT}
	for _, f := range formats {
		t.Run(f, func(t *testing.T) {
			data, ct, err := FormatResult(nil, f)
			if err == nil {
				t.Errorf("expected error for nil result with format %q, got nil", f)
			}
			if data != nil {
				t.Errorf("expected nil data for nil result, got %d bytes", len(data))
			}
			if ct != "" {
				t.Errorf("expected empty content-type for nil result, got %q", ct)
			}
		})
	}
}

// TestFormatResult_emptySegments verifies minimal valid output when the result
// has zero segments, covering formats not tested in the existing
// TestFormatResult_EmptySegments.
func TestFormatResult_emptySegments(t *testing.T) {
	result := &engine.Result{
		Language: "en",
		Duration: 0.0,
	}

	// Text format should return an empty byte slice.
	textData, ct, err := FormatResult(result, FormatText)
	if err != nil {
		t.Fatalf("text format with empty segments: %v", err)
	}
	if ct != ContentTypeText {
		t.Errorf("text content-type = %q, want %q", ct, ContentTypeText)
	}
	if string(textData) != "" {
		t.Errorf("expected empty text output, got %q", string(textData))
	}

	// Verbose JSON should produce valid JSON with empty segments array.
	verboseData, ct, err := FormatResult(result, FormatVerboseJSON)
	if err != nil {
		t.Fatalf("verbose_json format with empty segments: %v", err)
	}
	if ct != ContentTypeJSON {
		t.Errorf("verbose_json content-type = %q, want %q", ct, ContentTypeJSON)
	}
	var verbose map[string]interface{}
	if err := json.Unmarshal(verboseData, &verbose); err != nil {
		t.Fatalf("invalid verbose JSON: %v", err)
	}
	segments, ok := verbose["segments"].([]interface{})
	if !ok {
		t.Fatal("verbose JSON missing 'segments' array")
	}
	if len(segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segments))
	}
	if verbose["text"] != "" {
		t.Errorf("expected empty text in verbose JSON, got %q", verbose["text"])
	}
	// Words should be absent (omitempty) or null.
	if w, exists := verbose["words"]; exists && w != nil {
		t.Errorf("expected words to be absent or null, got %v", w)
	}
}

// TestFormatResult_invalidFormat verifies that a variety of unknown format
// strings all produce an error.
func TestFormatResult_invalidFormat(t *testing.T) {
	result := sampleResult()
	invalids := []string{"xml", "csv", "mp3", "", "JSON", "SRT", "verbose-json", " json"}
	for _, f := range invalids {
		t.Run(f, func(t *testing.T) {
			_, _, err := FormatResult(result, f)
			if err == nil {
				t.Errorf("expected error for invalid format %q, got nil", f)
			}
		})
	}
}

// TestIsValidFormat_edgeCases supplements the existing TestIsValidFormat with
// additional edge cases: mixed case, whitespace-padded, and empty string.
func TestIsValidFormat_edgeCases(t *testing.T) {
	// All canonical formats must be valid.
	for _, f := range []string{"json", "verbose_json", "text", "srt", "vtt"} {
		if !IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = false, want true", f)
		}
	}
	// Invalid formats -- edge cases the original test does not cover.
	for _, f := range []string{
		" ", "JSON", "VTT", "SRT", "Text",
		"verbose-json", " json", "json ", "verbose_JSON",
	} {
		if IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = true, want false", f)
		}
	}
}

// TestFormatSRT_multipleSegments builds a three-segment result and verifies
// SRT numbering, timestamp formatting, and segment ordering.
func TestFormatSRT_multipleSegments(t *testing.T) {
	result := &engine.Result{
		Language: "en",
		Duration: 15.0,
		Segments: []engine.Segment{
			{ID: 0, Start: 0.0, End: 3.5, Text: "First segment"},
			{ID: 1, Start: 4.0, End: 7.25, Text: "Second segment"},
			{ID: 2, Start: 8.0, End: 14.999, Text: "Third segment"},
		},
	}

	data := FormatSRTResponse(result)
	srt := string(data)

	// Verify 1-based sequence numbers for all three segments.
	for _, seq := range []string{"1\n", "2\n", "3\n"} {
		if !strings.Contains(srt, seq) {
			t.Errorf("SRT missing sequence number %q", seq)
		}
	}

	// Verify timecode format HH:MM:SS,mmm with --> separator.
	expectedTimecodes := []string{
		"00:00:00,000 --> 00:00:03,500",
		"00:00:04,000 --> 00:00:07,250",
		"00:00:08,000 --> 00:00:14,999",
	}
	for _, tc := range expectedTimecodes {
		if !strings.Contains(srt, tc) {
			t.Errorf("SRT missing timecode %q\nGot:\n%s", tc, srt)
		}
	}

	// Verify comma separator (not period) in all timecodes.
	lines := strings.Split(srt, "\n")
	for _, line := range lines {
		if strings.Contains(line, "-->") {
			if strings.Contains(line, ".") {
				t.Errorf("SRT timecode should use comma, not period: %q", line)
			}
		}
	}

	// Verify text content and ordering.
	idxFirst := strings.Index(srt, "First segment")
	idxSecond := strings.Index(srt, "Second segment")
	idxThird := strings.Index(srt, "Third segment")
	if idxFirst < 0 || idxSecond < 0 || idxThird < 0 {
		t.Fatalf("SRT missing segment text\nGot:\n%s", srt)
	}
	if !(idxFirst < idxSecond && idxSecond < idxThird) {
		t.Error("SRT segments are not in the correct order")
	}

	// Each segment block should be separated by a blank line.
	blocks := strings.Split(strings.TrimSpace(srt), "\n\n")
	if len(blocks) != 3 {
		t.Errorf("expected 3 SRT blocks, got %d\nGot:\n%s", len(blocks), srt)
	}
}

// TestFormatVTT_multipleSegments builds a three-segment result and verifies
// WebVTT header, cue format with period-separated milliseconds, and ordering.
func TestFormatVTT_multipleSegments(t *testing.T) {
	result := &engine.Result{
		Language: "en",
		Duration: 15.0,
		Segments: []engine.Segment{
			{ID: 0, Start: 0.0, End: 3.5, Text: "First cue"},
			{ID: 1, Start: 4.0, End: 7.25, Text: "Second cue"},
			{ID: 2, Start: 8.0, End: 14.999, Text: "Third cue"},
		},
	}

	data := FormatVTTResponse(result)
	vtt := string(data)

	// Must start with WEBVTT header.
	if !strings.HasPrefix(vtt, "WEBVTT\n") {
		t.Errorf("VTT must start with 'WEBVTT\\n', got prefix: %q", vtt[:min(20, len(vtt))])
	}

	// Verify period-based timecodes (HH:MM:SS.mmm).
	expectedTimecodes := []string{
		"00:00:00.000 --> 00:00:03.500",
		"00:00:04.000 --> 00:00:07.250",
		"00:00:08.000 --> 00:00:14.999",
	}
	for _, tc := range expectedTimecodes {
		if !strings.Contains(vtt, tc) {
			t.Errorf("VTT missing timecode %q\nGot:\n%s", tc, vtt)
		}
	}

	// VTT must NOT use comma in timecodes.
	lines := strings.Split(vtt, "\n")
	for _, line := range lines {
		if strings.Contains(line, "-->") {
			if strings.Contains(line, ",") {
				t.Errorf("VTT timecode should use period, not comma: %q", line)
			}
		}
	}

	// VTT should NOT contain sequence numbers (unlike SRT).
	// Sequence numbers would appear as standalone "1", "2", "3" lines.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "1" || trimmed == "2" || trimmed == "3" {
			t.Errorf("VTT should not contain SRT-style sequence number %q", trimmed)
		}
	}

	// Verify text content and ordering.
	idxFirst := strings.Index(vtt, "First cue")
	idxSecond := strings.Index(vtt, "Second cue")
	idxThird := strings.Index(vtt, "Third cue")
	if idxFirst < 0 || idxSecond < 0 || idxThird < 0 {
		t.Fatalf("VTT missing cue text\nGot:\n%s", vtt)
	}
	if !(idxFirst < idxSecond && idxSecond < idxThird) {
		t.Error("VTT cues are not in the correct order")
	}
}

// TestFormatVerboseJSON_withWords verifies that word-level timestamps appear
// in the verbose JSON output when the engine result contains per-word data.
func TestFormatVerboseJSON_withWords(t *testing.T) {
	result := &engine.Result{
		Language: "en",
		Duration: 5.0,
		Segments: []engine.Segment{
			{
				ID:    0,
				Start: 0.0,
				End:   2.5,
				Text:  "Hello world",
				Words: []engine.Word{
					{Word: "Hello", Start: 0.0, End: 1.0, Prob: 0.99},
					{Word: "world", Start: 1.1, End: 2.5, Prob: 0.95},
				},
			},
			{
				ID:    1,
				Start: 3.0,
				End:   5.0,
				Text:  "Good morning",
				Words: []engine.Word{
					{Word: "Good", Start: 3.0, End: 3.8, Prob: 0.98},
					{Word: "morning", Start: 3.9, End: 5.0, Prob: 0.97},
				},
			},
		},
	}

	data, err := FormatVerboseJSON_Response(result)
	if err != nil {
		t.Fatalf("FormatVerboseJSON_Response failed: %v", err)
	}

	var verbose map[string]interface{}
	if err := json.Unmarshal(data, &verbose); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Words array must be present and contain all 4 words.
	wordsRaw, ok := verbose["words"].([]interface{})
	if !ok {
		t.Fatal("verbose JSON missing 'words' array")
	}
	if len(wordsRaw) != 4 {
		t.Fatalf("expected 4 words, got %d", len(wordsRaw))
	}

	// Verify each word has the expected fields and values.
	expectedWords := []struct {
		word  string
		start float64
		end   float64
	}{
		{"Hello", 0.0, 1.0},
		{"world", 1.1, 2.5},
		{"Good", 3.0, 3.8},
		{"morning", 3.9, 5.0},
	}

	for i, ew := range expectedWords {
		wm, ok := wordsRaw[i].(map[string]interface{})
		if !ok {
			t.Fatalf("word[%d] is not an object", i)
		}
		if wm["word"] != ew.word {
			t.Errorf("word[%d].word = %q, want %q", i, wm["word"], ew.word)
		}
		if wm["start"] != ew.start {
			t.Errorf("word[%d].start = %v, want %v", i, wm["start"], ew.start)
		}
		if wm["end"] != ew.end {
			t.Errorf("word[%d].end = %v, want %v", i, wm["end"], ew.end)
		}
	}

	// Segments must also be present.
	segments, ok := verbose["segments"].([]interface{})
	if !ok {
		t.Fatal("verbose JSON missing 'segments' array")
	}
	if len(segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(segments))
	}
}

// TestFormatText_concatenation verifies that multiple segments are joined with
// spaces and that leading/trailing whitespace within segments is preserved in
// the concatenated output (since FullText does raw concatenation).
func TestFormatText_concatenation(t *testing.T) {
	tests := []struct {
		name     string
		segments []engine.Segment
		want     string
	}{
		{
			name:     "single segment",
			segments: []engine.Segment{{ID: 0, Start: 0, End: 1, Text: "Hello"}},
			want:     "Hello",
		},
		{
			name: "two segments joined by space",
			segments: []engine.Segment{
				{ID: 0, Start: 0, End: 1, Text: "Hello"},
				{ID: 1, Start: 1, End: 2, Text: "world"},
			},
			want: "Hello world",
		},
		{
			name: "three segments joined by spaces",
			segments: []engine.Segment{
				{ID: 0, Start: 0, End: 1, Text: "One"},
				{ID: 1, Start: 1, End: 2, Text: "Two"},
				{ID: 2, Start: 2, End: 3, Text: "Three"},
			},
			want: "One Two Three",
		},
		{
			name:     "no segments produces empty string",
			segments: nil,
			want:     "",
		},
		{
			name: "segments with leading spaces preserved in join",
			segments: []engine.Segment{
				{ID: 0, Start: 0, End: 1, Text: " Hello"},
				{ID: 1, Start: 1, End: 2, Text: " world"},
			},
			want: " Hello  world",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &engine.Result{
				Language: "en",
				Duration: 3.0,
				Segments: tc.segments,
			}
			data := FormatTextResponse(result)
			got := string(data)
			if got != tc.want {
				t.Errorf("FormatTextResponse = %q, want %q", got, tc.want)
			}
		})
	}
}
