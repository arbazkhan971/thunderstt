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
