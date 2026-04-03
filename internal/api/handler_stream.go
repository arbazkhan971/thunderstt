package api

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/arbaz/thunderstt/internal/engine"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  16 * 1024,
	WriteBufferSize: 16 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // permissive for dev; tighten for production
	},
}

// streamConfig is sent by the client as the first text frame.
type streamConfig struct {
	Model          string `json:"model"`
	Language       string `json:"language"`
	WordTimestamps bool   `json:"word_timestamps"`
}

// streamMessage is the envelope for all server-sent messages.
type streamMessage struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Segments []engine.Segment `json:"segments,omitempty"`
	Duration float64          `json:"duration,omitempty"`
	Message  string           `json:"message,omitempty"`
}

// HandleStream implements the WebSocket /v1/audio/stream endpoint.
// Clients send a config frame, then stream audio chunks, then send a stop
// frame. The server responds with partial transcriptions and a final result.
func (s *Server) HandleStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := reqLogger(ctx)

	if s.pipeline == nil || !s.pipeline.Ready() {
		WriteError(w, http.StatusServiceUnavailable, "model is not loaded; server is not ready")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("websocket upgrade failed")
		return
	}
	defer conn.Close()

	// Set read deadline for the config frame.
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read config frame.
	cfg := streamConfig{
		Model: "auto",
	}
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		sendStreamError(conn, "failed to read config frame: "+err.Error())
		return
	}
	if msgType == websocket.TextMessage {
		if err := json.Unmarshal(msg, &cfg); err != nil {
			sendStreamError(conn, "invalid config JSON: "+err.Error())
			return
		}
	}

	logger.Info().
		Str("model", cfg.Model).
		Str("language", cfg.Language).
		Bool("word_timestamps", cfg.WordTimestamps).
		Msg("stream session started")

	// Collect audio samples from binary frames.
	var (
		mu      sync.Mutex
		samples []float32
	)

	// Remove the deadline for audio streaming — client may take arbitrary time.
	conn.SetReadDeadline(time.Time{})

	// Read audio frames until stop signal or connection close.
	for {
		msgType, msg, err = conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Warn().Err(err).Msg("websocket read error")
			}
			break
		}

		switch msgType {
		case websocket.BinaryMessage:
			// Interpret binary data as little-endian float32 PCM samples.
			floats := bytesToFloat32(msg)
			mu.Lock()
			samples = append(samples, floats...)
			mu.Unlock()

			// Send partial transcription every ~2 seconds of audio (32000 samples at 16kHz).
			mu.Lock()
			sampleCount := len(samples)
			mu.Unlock()
			if sampleCount > 0 && sampleCount%32000 < len(floats) {
				go func(audio []float32) {
					opts := engine.Options{
						Language:       cfg.Language,
						WordTimestamps: cfg.WordTimestamps,
					}
					result, err := s.pipeline.ProcessAudio(audio, 16000)
					if err != nil {
						return
					}
					_ = opts // language/timestamps handled by pipeline
					partial := streamMessage{
						Type: "partial",
						Text: result.FullText(),
					}
					mu.Lock()
					_ = conn.WriteJSON(partial)
					mu.Unlock()
				}(append([]float32(nil), samples...))
			}

		case websocket.TextMessage:
			// Check for stop signal.
			var ctrl struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg, &ctrl); err == nil && ctrl.Type == "stop" {
				goto finalize
			}
			// If it's another config update, could handle here.
		}
	}

finalize:
	mu.Lock()
	finalSamples := samples
	mu.Unlock()

	if len(finalSamples) == 0 {
		sendStreamError(conn, "no audio data received")
		return
	}

	// Run final transcription on all collected audio.
	opts := engine.Options{
		Language:       cfg.Language,
		WordTimestamps: cfg.WordTimestamps,
	}
	result, err := s.pipeline.ProcessAudio(finalSamples, 16000)
	if err != nil {
		sendStreamError(conn, "transcription failed: "+err.Error())
		return
	}
	_ = opts

	final := streamMessage{
		Type:     "final",
		Text:     result.FullText(),
		Segments: result.Segments,
		Duration: result.Duration,
	}
	conn.WriteJSON(final)

	logger.Info().
		Float64("duration", result.Duration).
		Int("segments", len(result.Segments)).
		Msg("stream session completed")
}

// bytesToFloat32 converts a byte slice to float32 samples (little-endian IEEE 754).
func bytesToFloat32(data []byte) []float32 {
	n := len(data) / 4
	if n == 0 {
		return nil
	}
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples
}

// sendStreamError sends an error message over the WebSocket connection.
func sendStreamError(conn *websocket.Conn, msg string) {
	_ = conn.WriteJSON(streamMessage{
		Type:    "error",
		Message: msg,
	})
}
