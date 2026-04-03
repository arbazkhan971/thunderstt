package api

import (
	"bytes"
	"testing"
)

func TestValidateAudioFilename(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"test.wav", false},
		{"test.mp3", false},
		{"test.flac", false},
		{"test.ogg", false},
		{"test.m4a", false},
		{"test.webm", false},
		{"test.aac", false},
		{"test.opus", false},
		{"test.WAV", false}, // case insensitive
		{"test.txt", true},
		{"test.exe", true},
		{"noextension", true},
		{"test.py", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAudioFilename(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAudioFilename(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAudioMagicBytes(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantFormat string
		wantErr    bool
	}{
		{"wav", []byte("RIFF\x00\x00\x00\x00WAVE"), "wav", false},
		{"mp3_id3", []byte("ID3\x03\x00\x00\x00\x00\x00\x00\x00\x00"), "mp3", false},
		{"mp3_sync", []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00}, "mp3", false},
		{"flac", []byte("fLaC\x00\x00\x00\x00\x00\x00\x00\x00"), "flac", false},
		{"ogg", []byte("OggS\x00\x00\x00\x00\x00\x00\x00\x00"), "ogg", false},
		{"unknown", []byte("UNKNOWNDATA!"), "unknown", false},
		{"too_short", []byte{0x00}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			format, _, err := ValidateAudioMagicBytes(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
		})
	}
}
