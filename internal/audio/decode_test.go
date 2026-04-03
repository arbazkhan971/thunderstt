//go:build !cgo

package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

// buildWAV constructs a minimal valid WAV file in memory.
// format: 1=PCM, 3=IEEE float.  bitsPerSample: 8,16,24,32.
func buildWAV(sampleRate uint32, numChannels uint16, bitsPerSample uint16, audioFormat uint16, pcmData []byte) []byte {
	var buf bytes.Buffer

	dataSize := uint32(len(pcmData))
	fmtChunkSize := uint32(16)
	// RIFF header: 4 (WAVE) + 8+fmtChunkSize (fmt chunk) + 8+dataSize (data chunk)
	riffSize := 4 + (8 + fmtChunkSize) + (8 + dataSize)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, riffSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, fmtChunkSize)
	binary.Write(&buf, binary.LittleEndian, audioFormat)                                              // AudioFormat
	binary.Write(&buf, binary.LittleEndian, numChannels)                                              // NumChannels
	binary.Write(&buf, binary.LittleEndian, sampleRate)                                               // SampleRate
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8
	binary.Write(&buf, binary.LittleEndian, byteRate)                                                 // ByteRate
	blockAlign := numChannels * bitsPerSample / 8
	binary.Write(&buf, binary.LittleEndian, blockAlign)                                               // BlockAlign
	binary.Write(&buf, binary.LittleEndian, bitsPerSample)                                            // BitsPerSample

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(pcmData)

	return buf.Bytes()
}

// make16bitPCMSamples creates a byte slice of 16-bit LE signed PCM from float32 values.
func make16bitPCMSamples(vals []float32) []byte {
	buf := make([]byte, len(vals)*2)
	for i, v := range vals {
		s := int16(v * 32767)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestDecodeReader_WAV_16bit(t *testing.T) {
	// Produce a few samples of 16-bit PCM 16kHz mono WAV.
	rawSamples := []float32{0.0, 0.5, -0.5, 0.25, -0.25}
	pcmBytes := make16bitPCMSamples(rawSamples)
	wav := buildWAV(16000, 1, 16, 1, pcmBytes)

	samples, rate, err := DecodeReader(bytes.NewReader(wav), FormatWAV)
	if err != nil {
		t.Fatalf("DecodeReader WAV failed: %v", err)
	}

	if rate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", rate)
	}

	if len(samples) != len(rawSamples) {
		t.Fatalf("expected %d samples, got %d", len(rawSamples), len(samples))
	}

	// Check values are close (int16 quantization introduces small errors).
	for i, want := range rawSamples {
		got := samples[i]
		if diff := math.Abs(float64(got - want)); diff > 0.001 {
			t.Errorf("sample[%d]: want %.4f, got %.4f (diff %.6f)", i, want, got, diff)
		}
	}
}

func TestDecodeReader_WAV_8bit(t *testing.T) {
	// 8-bit PCM: unsigned, silence at 128.
	// Silence sample: 128 -> (128-128)/128 = 0.0
	// Positive sample: 192 -> (192-128)/128 = 0.5
	pcmData := []byte{128, 192, 64} // 0.0, 0.5, -0.5
	wav := buildWAV(16000, 1, 8, 1, pcmData)

	samples, rate, err := DecodeReader(bytes.NewReader(wav), FormatWAV)
	if err != nil {
		t.Fatalf("DecodeReader WAV 8-bit failed: %v", err)
	}
	if rate != 16000 {
		t.Errorf("expected rate 16000, got %d", rate)
	}
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	expected := []float32{0.0, 0.5, -0.5}
	for i, want := range expected {
		if diff := math.Abs(float64(samples[i] - want)); diff > 0.01 {
			t.Errorf("sample[%d]: want %.2f, got %.2f", i, want, samples[i])
		}
	}
}

func TestDecodeReader_WAV_32bitFloat(t *testing.T) {
	// Build 32-bit float WAV (audioFormat = 3).
	rawSamples := []float32{0.0, 0.75, -0.75, 1.0, -1.0}
	pcmData := make([]byte, len(rawSamples)*4)
	for i, v := range rawSamples {
		binary.LittleEndian.PutUint32(pcmData[i*4:], math.Float32bits(v))
	}
	wav := buildWAV(44100, 1, 32, 3, pcmData) // 32-bit float, 44.1kHz

	samples, rate, err := DecodeReader(bytes.NewReader(wav), FormatWAV)
	if err != nil {
		t.Fatalf("DecodeReader WAV 32-bit float failed: %v", err)
	}
	if rate != 44100 {
		t.Errorf("expected rate 44100, got %d", rate)
	}
	if len(samples) != len(rawSamples) {
		t.Fatalf("expected %d samples, got %d", len(rawSamples), len(samples))
	}
	for i, want := range rawSamples {
		if samples[i] != want {
			t.Errorf("sample[%d]: want %f, got %f", i, want, samples[i])
		}
	}
}

func TestDecodeReader_WAV_Stereo(t *testing.T) {
	// 2-channel (stereo) 16-bit PCM.
	// Two frames: frame0 = (L=0.5, R=-0.5), frame1 = (L=0.25, R=-0.25)
	interleaved := []float32{0.5, -0.5, 0.25, -0.25}
	pcmBytes := make16bitPCMSamples(interleaved)
	wav := buildWAV(16000, 2, 16, 1, pcmBytes)

	samples, rate, err := DecodeReader(bytes.NewReader(wav), FormatWAV)
	if err != nil {
		t.Fatalf("DecodeReader stereo WAV failed: %v", err)
	}
	if rate != 16000 {
		t.Errorf("expected rate 16000, got %d", rate)
	}
	// Should return all 4 interleaved samples.
	if len(samples) != 4 {
		t.Fatalf("expected 4 interleaved samples, got %d", len(samples))
	}
}

func TestDecodeReader_InvalidWAV(t *testing.T) {
	// Not a valid WAV: just random bytes.
	_, _, err := DecodeReader(bytes.NewReader([]byte("not a wav file")), FormatWAV)
	if err == nil {
		t.Fatal("expected error for invalid WAV, got nil")
	}
	if !errors.Is(err, ErrInvalidWAV) {
		t.Errorf("expected ErrInvalidWAV, got: %v", err)
	}
}

func TestDecodeReader_UnsupportedFormat(t *testing.T) {
	_, _, err := DecodeReader(bytes.NewReader(nil), "aac")
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got: %v", err)
	}
}

func TestDetectFormat_WAV(t *testing.T) {
	// Build minimal WAV magic bytes: "RIFF" + 4 bytes size + "WAVE"
	magic := []byte("RIFF\x00\x00\x00\x00WAVE")
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	format, err := detectFormat(r)
	if err != nil {
		t.Fatalf("detectFormat WAV failed: %v", err)
	}
	if format != FormatWAV {
		t.Errorf("expected format %q, got %q", FormatWAV, format)
	}
}

func TestDetectFormat_FLAC(t *testing.T) {
	magic := []byte("fLaC\x00\x00\x00\x00\x00\x00\x00\x00")
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	format, err := detectFormat(r)
	if err != nil {
		t.Fatalf("detectFormat FLAC failed: %v", err)
	}
	if format != FormatFLAC {
		t.Errorf("expected format %q, got %q", FormatFLAC, format)
	}
}

func TestDetectFormat_OGG(t *testing.T) {
	magic := []byte("OggS\x00\x00\x00\x00\x00\x00\x00\x00")
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	format, err := detectFormat(r)
	if err != nil {
		t.Fatalf("detectFormat OGG failed: %v", err)
	}
	if format != FormatOGG {
		t.Errorf("expected format %q, got %q", FormatOGG, format)
	}
}

func TestDetectFormat_MP3_ID3(t *testing.T) {
	magic := []byte("ID3\x04\x00\x00\x00\x00\x00\x00\x00\x00")
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	format, err := detectFormat(r)
	if err != nil {
		t.Fatalf("detectFormat MP3 ID3 failed: %v", err)
	}
	if format != FormatMP3 {
		t.Errorf("expected format %q, got %q", FormatMP3, format)
	}
}

func TestDetectFormat_MP3_FrameSync(t *testing.T) {
	// MP3 frame sync: 0xFF followed by byte with top 3 bits set (0xE0 mask).
	magic := []byte{0xFF, 0xFB, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	format, err := detectFormat(r)
	if err != nil {
		t.Fatalf("detectFormat MP3 frame sync failed: %v", err)
	}
	if format != FormatMP3 {
		t.Errorf("expected format %q, got %q", FormatMP3, format)
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	magic := []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	r := io.NewSectionReader(readerAtFromBytes(magic), 0, int64(len(magic)))

	_, err := detectFormat(r)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got: %v", err)
	}
}

func TestFormatFromExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"audio.wav", FormatWAV},
		{"audio.WAVE", FormatWAV},
		{"song.mp3", FormatMP3},
		{"music.flac", FormatFLAC},
		{"speech.ogg", FormatOGG},
		{"speech.oga", FormatOGG},
		{"file.m4a", ""},
		{"noext", ""},
	}
	for _, tt := range tests {
		got := formatFromExt(tt.path)
		if got != tt.want {
			t.Errorf("formatFromExt(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
