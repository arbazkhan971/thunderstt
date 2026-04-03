package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
)

const (
	// ffmpegOutputRate is the sample rate ffmpeg is told to produce.
	ffmpegOutputRate = 16000
)

// IsFFmpegAvailable returns true if the ffmpeg binary can be found on the
// system PATH. The result is not cached; callers should cache if needed.
func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// DecodeWithFFmpeg shells out to ffmpeg to decode an arbitrary audio file
// into 16 kHz mono float32 PCM. This is used as a fallback when native Go
// decoders do not support the file format (e.g. M4A, WebM, AAC).
//
// ffmpeg must be available on PATH; if it is not, an error is returned.
//
// The returned samples are mono 16 kHz float32le, ready for the pipeline.
func DecodeWithFFmpeg(path string) ([]float32, int, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, 0, fmt.Errorf("audio: ffmpeg not found on PATH: %w", err)
	}

	// Build the ffmpeg command:
	//   -i <path>            input file
	//   -f f32le             output raw 32-bit float little-endian
	//   -acodec pcm_f32le    force float32 PCM codec
	//   -ar 16000            resample to 16 kHz
	//   -ac 1                mix down to mono
	//   -v error             suppress noisy banner/info
	//   pipe:1               write to stdout
	cmd := exec.Command(
		ffmpegPath,
		"-i", path,
		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ar", fmt.Sprintf("%d", ffmpegOutputRate),
		"-ac", "1",
		"-v", "error",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := stderr.String()
		if detail == "" {
			detail = err.Error()
		}
		return nil, 0, fmt.Errorf("audio: ffmpeg failed: %s", detail)
	}

	raw := stdout.Bytes()
	if len(raw) == 0 {
		return nil, 0, fmt.Errorf("audio: ffmpeg produced no output for %s", path)
	}

	// Convert raw float32le bytes to []float32.
	numSamples := len(raw) / 4
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		off := i * 4
		bits := binary.LittleEndian.Uint32(raw[off : off+4])
		samples[i] = math.Float32frombits(bits)
	}

	return samples, ffmpegOutputRate, nil
}
