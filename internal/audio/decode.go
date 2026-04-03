// Package audio provides audio file decoding, resampling, and format
// conversion for the thunderstt pipeline.
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	gomp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
)

// Supported format identifiers.
const (
	FormatWAV  = "wav"
	FormatMP3  = "mp3"
	FormatFLAC = "flac"
	FormatOGG  = "ogg"
)

var (
	// ErrUnsupportedFormat is returned when the audio format cannot be decoded.
	ErrUnsupportedFormat = errors.New("audio: unsupported format")

	// ErrInvalidWAV is returned when a WAV file is malformed.
	ErrInvalidWAV = errors.New("audio: invalid WAV file")
)

// DecodeFile opens the file at path and returns interleaved float32 PCM
// samples, the sample rate, and any error. The format is inferred from the
// file extension; if that fails, magic bytes are inspected.
func DecodeFile(path string) ([]float32, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("audio: open %s: %w", path, err)
	}
	defer f.Close()

	format := formatFromExt(path)
	if format == "" {
		det, err := detectFormat(f)
		if err != nil {
			return nil, 0, fmt.Errorf("audio: detect format: %w", err)
		}
		format = det
		// Seek back to start after peeking at magic bytes.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, 0, fmt.Errorf("audio: seek: %w", err)
		}
	}

	return DecodeReader(f, format)
}

// DecodeReader decodes audio from r in the given format. The format string
// should be one of the Format* constants (e.g. "wav", "mp3", "flac", "ogg").
func DecodeReader(r io.Reader, format string) ([]float32, int, error) {
	switch strings.ToLower(format) {
	case FormatWAV:
		return decodeWAV(r)
	case FormatMP3:
		return decodeMP3(r)
	case FormatFLAC:
		return decodeFLAC(r)
	case FormatOGG:
		return decodeOGG(r)
	default:
		return nil, 0, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

// ---------------------------------------------------------------------------
// WAV decoding (from scratch)
// ---------------------------------------------------------------------------

// wavHeader holds the parsed fields we care about from a RIFF/WAVE file.
type wavHeader struct {
	AudioFormat   uint16 // 1 = PCM, 3 = IEEE float
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
}

const (
	wavFormatPCM   = 1
	wavFormatFloat = 3
)

// decodeWAV parses a WAV (RIFF WAVE) stream and returns float32 samples.
// Supported sample formats: 8/16/24/32-bit integer PCM and 32-bit float.
func decodeWAV(r io.Reader) ([]float32, int, error) {
	// --- RIFF header ---
	var riffID [4]byte
	if err := binary.Read(r, binary.LittleEndian, &riffID); err != nil {
		return nil, 0, fmt.Errorf("%w: read RIFF ID: %v", ErrInvalidWAV, err)
	}
	if string(riffID[:]) != "RIFF" {
		return nil, 0, fmt.Errorf("%w: missing RIFF tag", ErrInvalidWAV)
	}

	var fileSize uint32
	if err := binary.Read(r, binary.LittleEndian, &fileSize); err != nil {
		return nil, 0, fmt.Errorf("%w: read file size: %v", ErrInvalidWAV, err)
	}

	var waveID [4]byte
	if err := binary.Read(r, binary.LittleEndian, &waveID); err != nil {
		return nil, 0, fmt.Errorf("%w: read WAVE ID: %v", ErrInvalidWAV, err)
	}
	if string(waveID[:]) != "WAVE" {
		return nil, 0, fmt.Errorf("%w: missing WAVE tag", ErrInvalidWAV)
	}

	// --- Walk chunks until we find "fmt " and "data" ---
	var hdr wavHeader
	var hdrFound bool
	var samples []float32

	for {
		var chunkID [4]byte
		if err := binary.Read(r, binary.LittleEndian, &chunkID); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, 0, fmt.Errorf("%w: read chunk ID: %v", ErrInvalidWAV, err)
		}

		var chunkSize uint32
		if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
			return nil, 0, fmt.Errorf("%w: read chunk size: %v", ErrInvalidWAV, err)
		}

		id := string(chunkID[:])

		switch id {
		case "fmt ":
			if err := parseFmtChunk(r, chunkSize, &hdr); err != nil {
				return nil, 0, err
			}
			hdrFound = true

		case "data":
			if !hdrFound {
				return nil, 0, fmt.Errorf("%w: data chunk before fmt chunk", ErrInvalidWAV)
			}
			var err error
			samples, err = readDataChunk(r, chunkSize, &hdr)
			if err != nil {
				return nil, 0, err
			}
			// We have what we need; stop reading.
			return samples, int(hdr.SampleRate), nil

		default:
			// Skip unknown chunks. Chunks are word-aligned: if the size is
			// odd, an extra padding byte follows.
			skip := int64(chunkSize)
			if chunkSize%2 != 0 {
				skip++
			}
			if rs, ok := r.(io.Seeker); ok {
				if _, err := rs.Seek(skip, io.SeekCurrent); err != nil {
					return nil, 0, fmt.Errorf("%w: seek past chunk %q: %v", ErrInvalidWAV, id, err)
				}
			} else {
				if _, err := io.CopyN(io.Discard, r, skip); err != nil {
					return nil, 0, fmt.Errorf("%w: skip chunk %q: %v", ErrInvalidWAV, id, err)
				}
			}
		}
	}

	if samples != nil {
		return samples, int(hdr.SampleRate), nil
	}
	return nil, 0, fmt.Errorf("%w: no data chunk found", ErrInvalidWAV)
}

// parseFmtChunk reads the "fmt " sub-chunk into hdr.
func parseFmtChunk(r io.Reader, size uint32, hdr *wavHeader) error {
	if size < 16 {
		return fmt.Errorf("%w: fmt chunk too short (%d bytes)", ErrInvalidWAV, size)
	}

	if err := binary.Read(r, binary.LittleEndian, &hdr.AudioFormat); err != nil {
		return fmt.Errorf("%w: read AudioFormat: %v", ErrInvalidWAV, err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.NumChannels); err != nil {
		return fmt.Errorf("%w: read NumChannels: %v", ErrInvalidWAV, err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.SampleRate); err != nil {
		return fmt.Errorf("%w: read SampleRate: %v", ErrInvalidWAV, err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.ByteRate); err != nil {
		return fmt.Errorf("%w: read ByteRate: %v", ErrInvalidWAV, err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.BlockAlign); err != nil {
		return fmt.Errorf("%w: read BlockAlign: %v", ErrInvalidWAV, err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.BitsPerSample); err != nil {
		return fmt.Errorf("%w: read BitsPerSample: %v", ErrInvalidWAV, err)
	}

	// Validate format.
	switch hdr.AudioFormat {
	case wavFormatPCM:
		switch hdr.BitsPerSample {
		case 8, 16, 24, 32:
		default:
			return fmt.Errorf("%w: unsupported PCM bit depth %d", ErrInvalidWAV, hdr.BitsPerSample)
		}
	case wavFormatFloat:
		if hdr.BitsPerSample != 32 {
			return fmt.Errorf("%w: float WAV must be 32-bit, got %d", ErrInvalidWAV, hdr.BitsPerSample)
		}
	default:
		return fmt.Errorf("%w: unsupported audio format tag %d (only PCM=1 and float=3)", ErrInvalidWAV, hdr.AudioFormat)
	}

	// Skip any extra bytes in an extended fmt chunk.
	if size > 16 {
		extra := int64(size - 16)
		if rs, ok := r.(io.Seeker); ok {
			if _, err := rs.Seek(extra, io.SeekCurrent); err != nil {
				return fmt.Errorf("%w: seek past fmt extras: %v", ErrInvalidWAV, err)
			}
		} else {
			if _, err := io.CopyN(io.Discard, r, extra); err != nil {
				return fmt.Errorf("%w: skip fmt extras: %v", ErrInvalidWAV, err)
			}
		}
	}

	return nil
}

// readDataChunk reads raw PCM/float bytes and converts them to float32.
func readDataChunk(r io.Reader, size uint32, hdr *wavHeader) ([]float32, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, fmt.Errorf("%w: read data chunk: %v", ErrInvalidWAV, err)
	}

	bytesPerSample := int(hdr.BitsPerSample) / 8
	if bytesPerSample == 0 {
		return nil, fmt.Errorf("%w: zero bytes per sample", ErrInvalidWAV)
	}
	numSamples := len(raw) / bytesPerSample
	samples := make([]float32, numSamples)

	switch {
	case hdr.AudioFormat == wavFormatPCM && hdr.BitsPerSample == 8:
		// 8-bit PCM is unsigned: 0-255, silence at 128.
		for i := 0; i < numSamples; i++ {
			samples[i] = (float32(raw[i]) - 128.0) / 128.0
		}

	case hdr.AudioFormat == wavFormatPCM && hdr.BitsPerSample == 16:
		for i := 0; i < numSamples; i++ {
			off := i * 2
			v := int16(binary.LittleEndian.Uint16(raw[off : off+2]))
			samples[i] = float32(v) / 32768.0
		}

	case hdr.AudioFormat == wavFormatPCM && hdr.BitsPerSample == 24:
		for i := 0; i < numSamples; i++ {
			off := i * 3
			// Assemble 24-bit signed integer (little-endian).
			v := int32(raw[off]) | int32(raw[off+1])<<8 | int32(raw[off+2])<<16
			// Sign-extend from 24-bit.
			if v&0x800000 != 0 {
				v |= ^0xFFFFFF // set upper bits for negative
			}
			samples[i] = float32(v) / 8388608.0 // 2^23
		}

	case hdr.AudioFormat == wavFormatPCM && hdr.BitsPerSample == 32:
		for i := 0; i < numSamples; i++ {
			off := i * 4
			v := int32(binary.LittleEndian.Uint32(raw[off : off+4]))
			samples[i] = float32(v) / 2147483648.0 // 2^31
		}

	case hdr.AudioFormat == wavFormatFloat && hdr.BitsPerSample == 32:
		for i := 0; i < numSamples; i++ {
			off := i * 4
			bits := binary.LittleEndian.Uint32(raw[off : off+4])
			samples[i] = math.Float32frombits(bits)
		}

	default:
		return nil, fmt.Errorf("%w: unhandled format=%d bits=%d", ErrInvalidWAV, hdr.AudioFormat, hdr.BitsPerSample)
	}

	return samples, nil
}

// ---------------------------------------------------------------------------
// MP3 decoding (via go-mp3)
// ---------------------------------------------------------------------------

func decodeMP3(r io.Reader) ([]float32, int, error) {
	// go-mp3 requires an io.ReadSeeker for seeking within the stream. If the
	// reader is not seekable, buffer the whole thing first.
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, 0, fmt.Errorf("audio: read MP3 data: %w", err)
		}
		rs = io.NewSectionReader(
			readerAtFromBytes(data), 0, int64(len(data)),
		)
	}

	dec, err := gomp3.NewDecoder(rs)
	if err != nil {
		return nil, 0, fmt.Errorf("audio: MP3 decoder init: %w", err)
	}

	sampleRate := dec.SampleRate()

	// go-mp3 always outputs signed 16-bit stereo interleaved PCM.
	pcm, err := io.ReadAll(dec)
	if err != nil {
		return nil, 0, fmt.Errorf("audio: MP3 decode: %w", err)
	}

	numSamples := len(pcm) / 2 // 2 bytes per sample (16-bit)
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		off := i * 2
		v := int16(binary.LittleEndian.Uint16(pcm[off : off+2]))
		samples[i] = float32(v) / 32768.0
	}

	return samples, sampleRate, nil
}

// ---------------------------------------------------------------------------
// FLAC decoding
// ---------------------------------------------------------------------------

func decodeFLAC(r io.Reader) ([]float32, int, error) {
	// mewkiz/flac supports io.ReadSeeker via flac.Parse, but also has
	// flac.New for non-seekable readers. We'll use flac.New which accepts
	// an io.Reader.
	stream, err := flac.New(r)
	if err != nil {
		return nil, 0, fmt.Errorf("audio: FLAC open: %w", err)
	}
	defer stream.Close()

	sampleRate := int(stream.Info.SampleRate)
	bps := int(stream.Info.BitsPerSample)

	var samples []float32
	for {
		frame, err := stream.ParseNext()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("audio: FLAC decode frame: %w", err)
		}

		nChannels := len(frame.Subframes)
		if nChannels == 0 {
			continue
		}
		nSamples := len(frame.Subframes[0].Samples)

		// FLAC stores samples per channel; interleave them.
		for s := 0; s < nSamples; s++ {
			for ch := 0; ch < nChannels; ch++ {
				raw := frame.Subframes[ch].Samples[s]
				samples = append(samples, pcmIntToFloat32(raw, bps))
			}
		}
	}

	return samples, sampleRate, nil
}

// pcmIntToFloat32 converts a signed PCM integer with the given bit depth to
// float32 in the range [-1.0, 1.0).
func pcmIntToFloat32(v int32, bitsPerSample int) float32 {
	maxVal := float32(int64(1) << (bitsPerSample - 1))
	return float32(v) / maxVal
}

// ---------------------------------------------------------------------------
// OGG/Vorbis decoding
// ---------------------------------------------------------------------------

func decodeOGG(r io.Reader) ([]float32, int, error) {
	// oggvorbis.NewReader requires io.Reader.
	reader, err := oggvorbis.NewReader(r)
	if err != nil {
		return nil, 0, fmt.Errorf("audio: OGG open: %w", err)
	}

	sampleRate := int(reader.SampleRate())
	channels := int(reader.Channels())

	// Read all samples; oggvorbis already returns interleaved float32.
	buf := make([]float32, 8192)
	var samples []float32
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			samples = append(samples, buf[:n]...)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("audio: OGG decode: %w", err)
		}
	}

	// oggvorbis returns interleaved multi-channel samples; the caller will
	// use ToMono if needed, so we just return them as-is. We tag the channel
	// count in the sample rate (the caller knows how many channels via the
	// pipeline). Actually, we should just return the interleaved samples and
	// let the caller handle channel mixing. The sample rate is the per-
	// channel sample rate.
	_ = channels // Interleaved samples; caller uses ToMono with channel count.
	return samples, sampleRate, nil
}

// ---------------------------------------------------------------------------
// Format detection helpers
// ---------------------------------------------------------------------------

// formatFromExt returns the format string for common audio extensions.
func formatFromExt(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "wav", "wave":
		return FormatWAV
	case "mp3":
		return FormatMP3
	case "flac":
		return FormatFLAC
	case "ogg", "oga", "ogx":
		return FormatOGG
	default:
		return ""
	}
}

// detectFormat peeks at the first few bytes of r to identify the audio format
// via magic bytes. r must be an io.ReadSeeker so the caller can rewind.
func detectFormat(r io.ReadSeeker) (string, error) {
	var magic [12]byte
	n, err := r.Read(magic[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if n < 4 {
		return "", ErrUnsupportedFormat
	}

	// RIFF....WAVE
	if string(magic[0:4]) == "RIFF" && n >= 12 && string(magic[8:12]) == "WAVE" {
		return FormatWAV, nil
	}

	// FLAC magic: "fLaC"
	if string(magic[0:4]) == "fLaC" {
		return FormatFLAC, nil
	}

	// OGG magic: "OggS"
	if string(magic[0:4]) == "OggS" {
		return FormatOGG, nil
	}

	// MP3: ID3 tag or frame sync (0xFF 0xFB / 0xFF 0xF3 / 0xFF 0xF2).
	if string(magic[0:3]) == "ID3" {
		return FormatMP3, nil
	}
	if n >= 2 && magic[0] == 0xFF && (magic[1]&0xE0) == 0xE0 {
		return FormatMP3, nil
	}

	return "", ErrUnsupportedFormat
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// bytesReaderAt wraps a byte slice to satisfy io.ReaderAt.
type bytesReaderAt struct{ data []byte }

func (b *bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func readerAtFromBytes(data []byte) *bytesReaderAt {
	return &bytesReaderAt{data: data}
}
