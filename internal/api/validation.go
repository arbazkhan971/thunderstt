package api

import (
	"fmt"
	"io"
	"strings"
)

// allowedAudioExtensions lists file extensions the server accepts.
var allowedAudioExtensions = map[string]bool{
	".wav":  true,
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".m4a":  true,
	".webm": true,
	".aac":  true,
	".wma":  true,
	".opus": true,
}

// audioMagicBytes maps known magic byte prefixes to audio format names.
var audioMagicBytes = []struct {
	prefix []byte
	format string
}{
	{[]byte("RIFF"), "wav"},
	{[]byte("ID3"), "mp3"},
	{[]byte{0xFF, 0xFB}, "mp3"},
	{[]byte{0xFF, 0xF3}, "mp3"},
	{[]byte{0xFF, 0xF2}, "mp3"},
	{[]byte("fLaC"), "flac"},
	{[]byte("OggS"), "ogg"},
	{[]byte{0x00, 0x00, 0x00}, "m4a"}, // ftyp box (partial)
}

// ValidateAudioFilename checks that the filename has an allowed audio extension.
func ValidateAudioFilename(filename string) error {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return fmt.Errorf("file has no extension; supported formats: wav, mp3, flac, ogg, m4a, webm")
	}
	ext := strings.ToLower(filename[idx:])
	if !allowedAudioExtensions[ext] {
		return fmt.Errorf("unsupported file extension %q; supported: wav, mp3, flac, ogg, m4a, webm, aac, opus", ext)
	}
	return nil
}

// ValidateAudioMagicBytes reads the first few bytes of an audio stream and
// checks for known magic byte patterns. Returns the detected format name,
// the bytes read (to be prepended back), and any error.
func ValidateAudioMagicBytes(r io.Reader) (format string, header []byte, err error) {
	header = make([]byte, 12)
	n, err := io.ReadAtLeast(r, header, 4)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file header: %w", err)
	}
	header = header[:n]

	for _, magic := range audioMagicBytes {
		if len(header) >= len(magic.prefix) {
			match := true
			for i, b := range magic.prefix {
				if header[i] != b {
					match = false
					break
				}
			}
			if match {
				return magic.format, header, nil
			}
		}
	}

	// No known magic bytes found -- might still be valid (ffmpeg can handle it).
	return "unknown", header, nil
}
