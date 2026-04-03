package api

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

// newWSTestServer creates an httptest.Server that exposes the HandleStream
// endpoint directly, bypassing middleware that wraps ResponseWriter (which
// would break the WebSocket hijack).
func newWSTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Host: "127.0.0.1", Port: 8080, Model: "test",
		Workers: 2, LogLevel: "error", ModelsDir: t.TempDir(),
	}
	eng := engine.NewNoopEngine("test-noop")
	p := pipeline.New(eng)
	t.Cleanup(func() { p.Close() })
	srv := NewServer(p, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio/stream", srv.HandleStream)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func wsURL(ts *httptest.Server) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http") + "/v1/audio/stream"
}

// float32ToBytes converts float32 samples to little-endian byte slice.
func float32ToBytes(samples []float32) []byte {
	buf := make([]byte, len(samples)*4)
	for i, s := range samples {
		bits := math.Float32bits(s)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

// TestHandleStream_FullSession connects via WebSocket, sends a config frame,
// sends silent PCM audio, sends a stop frame, and verifies a "final" response.
func TestHandleStream_FullSession(t *testing.T) {
	ts := newWSTestServer(t)

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL(ts), nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}
	defer conn.Close()

	// Send config frame.
	configMsg, _ := json.Marshal(streamConfig{Model: "auto", Language: "en"})
	if err := conn.WriteMessage(websocket.TextMessage, configMsg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Send some silent audio (1 second at 16 kHz).
	silence := make([]float32, 16000)
	if err := conn.WriteMessage(websocket.BinaryMessage, float32ToBytes(silence)); err != nil {
		t.Fatalf("write audio: %v", err)
	}

	// Send stop.
	stopMsg, _ := json.Marshal(map[string]string{"type": "stop"})
	if err := conn.WriteMessage(websocket.TextMessage, stopMsg); err != nil {
		t.Fatalf("write stop: %v", err)
	}

	// Read final result.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read final: %v", err)
	}

	var result streamMessage
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	if result.Type != "final" {
		t.Fatalf("expected type \"final\", got %q (message: %s)", result.Type, string(msg))
	}
}

// TestHandleStream_NoAudio sends a config frame followed immediately by a stop
// frame without any audio data and expects an error message.
func TestHandleStream_NoAudio(t *testing.T) {
	ts := newWSTestServer(t)

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL(ts), nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}
	defer conn.Close()

	// Send config frame.
	configMsg, _ := json.Marshal(streamConfig{Model: "auto", Language: "en"})
	if err := conn.WriteMessage(websocket.TextMessage, configMsg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Send stop immediately (no audio).
	stopMsg, _ := json.Marshal(map[string]string{"type": "stop"})
	if err := conn.WriteMessage(websocket.TextMessage, stopMsg); err != nil {
		t.Fatalf("write stop: %v", err)
	}

	// Read error response.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var result streamMessage
	if err := json.Unmarshal(msg, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Type != "error" {
		t.Fatalf("expected type \"error\", got %q", result.Type)
	}
	if !strings.Contains(result.Message, "no audio data received") {
		t.Fatalf("expected error about no audio data, got: %q", result.Message)
	}
}

// TestHandleStream_ConnectionClose verifies the server does not panic when
// the client closes the connection without sending a stop frame.
func TestHandleStream_ConnectionClose(t *testing.T) {
	ts := newWSTestServer(t)

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL(ts), nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}

	// Send config frame.
	configMsg, _ := json.Marshal(streamConfig{Model: "auto", Language: "en"})
	if err := conn.WriteMessage(websocket.TextMessage, configMsg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Send some audio.
	silence := make([]float32, 8000) // 0.5 seconds at 16 kHz
	if err := conn.WriteMessage(websocket.BinaryMessage, float32ToBytes(silence)); err != nil {
		t.Fatalf("write audio: %v", err)
	}

	// Close connection abruptly without a stop frame.
	conn.Close()

	// Give the server a moment to handle the closed connection.
	time.Sleep(200 * time.Millisecond)

	// If we get here without panic, the test passes.
}

// TestBytesToFloat32 verifies the bytesToFloat32 helper round-trips correctly
// with known float32 values.
func TestBytesToFloat32(t *testing.T) {
	input := []float32{0.0, 1.0, -1.0, 0.5, math.SmallestNonzeroFloat32, math.MaxFloat32}
	raw := float32ToBytes(input)

	got := bytesToFloat32(raw)
	if len(got) != len(input) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(input))
	}
	for i := range input {
		if got[i] != input[i] {
			t.Errorf("sample[%d]: got %v, want %v", i, got[i], input[i])
		}
	}
}

// TestBytesToFloat32_Empty verifies that empty input returns nil.
func TestBytesToFloat32_Empty(t *testing.T) {
	got := bytesToFloat32(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}

	got = bytesToFloat32([]byte{})
	if got != nil {
		t.Fatalf("expected nil for empty slice, got %v", got)
	}

	// Fewer than 4 bytes should also return nil.
	got = bytesToFloat32([]byte{0x01, 0x02, 0x03})
	if got != nil {
		t.Fatalf("expected nil for 3-byte slice, got %v", got)
	}
}
