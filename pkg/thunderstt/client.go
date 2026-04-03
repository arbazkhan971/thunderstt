// Package thunderstt provides a Go client for the ThunderSTT speech-to-text API.
package thunderstt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client is a ThunderSTT API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new ThunderSTT client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option configures the client.
type Option func(*Client)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.APIKey = key }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.HTTPClient = hc }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.HTTPClient.Timeout = d }
}

// TranscribeRequest configures a transcription request.
type TranscribeRequest struct {
	// FilePath is the path to the audio file to transcribe.
	FilePath string
	// Reader is an alternative to FilePath — an io.Reader with audio data.
	Reader io.Reader
	// Filename is required when using Reader (for content type detection).
	Filename string
	// Model is the model ID (default: "auto").
	Model string
	// Language is an optional BCP-47 language hint.
	Language string
	// ResponseFormat is one of: json, verbose_json, text, srt, vtt.
	ResponseFormat string
	// TimestampGranularities specifies "word" and/or "segment".
	TimestampGranularities []string
}

// TranscribeResponse is the JSON response from the transcription API.
type TranscribeResponse struct {
	Text     string    `json:"text"`
	Language string    `json:"language,omitempty"`
	Duration float64   `json:"duration,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
	Words    []Word    `json:"words,omitempty"`
}

// Segment is a transcribed segment.
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Word is a transcribed word with timing.
type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Model represents a model in the listing.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the response from the models endpoint.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// HealthResponse is the response from the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime,omitempty"`
}

// VersionResponse is the response from the version endpoint.
type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Transcribe sends an audio file for transcription.
func (c *Client) Transcribe(req TranscribeRequest) (*TranscribeResponse, error) {
	return c.doTranscription("/v1/audio/transcriptions", req)
}

// Translate sends an audio file for translation to English.
func (c *Client) Translate(req TranscribeRequest) (*TranscribeResponse, error) {
	return c.doTranscription("/v1/audio/translations", req)
}

// doTranscription builds a multipart request and posts it to the given endpoint.
func (c *Client) doTranscription(endpoint string, req TranscribeRequest) (*TranscribeResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file.
	var filename string
	if req.FilePath != "" {
		filename = filepath.Base(req.FilePath)
		f, err := os.Open(req.FilePath)
		if err != nil {
			return nil, fmt.Errorf("thunderstt: open file: %w", err)
		}
		defer f.Close()
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, fmt.Errorf("thunderstt: create form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			return nil, fmt.Errorf("thunderstt: copy file: %w", err)
		}
	} else if req.Reader != nil {
		filename = req.Filename
		if filename == "" {
			filename = "audio.wav"
		}
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, fmt.Errorf("thunderstt: create form file: %w", err)
		}
		if _, err := io.Copy(part, req.Reader); err != nil {
			return nil, fmt.Errorf("thunderstt: copy reader: %w", err)
		}
	} else {
		return nil, fmt.Errorf("thunderstt: either FilePath or Reader must be set")
	}

	// Add fields.
	model := req.Model
	if model == "" {
		model = "auto"
	}
	writer.WriteField("model", model)

	if req.Language != "" {
		writer.WriteField("language", req.Language)
	}

	format := req.ResponseFormat
	if format == "" {
		format = "json"
	}
	writer.WriteField("response_format", format)

	for _, g := range req.TimestampGranularities {
		writer.WriteField("timestamp_granularities[]", g)
	}

	writer.Close()

	httpReq, err := http.NewRequest("POST", c.BaseURL+endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("thunderstt: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	c.setAuth(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("thunderstt: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("thunderstt: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result TranscribeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("thunderstt: decode response: %w", err)
	}
	return &result, nil
}

// Version returns the server's version information.
func (c *Client) Version() (*VersionResponse, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/version")
	if err != nil {
		return nil, fmt.Errorf("thunderstt: version check: %w", err)
	}
	defer resp.Body.Close()

	var result VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("thunderstt: decode response: %w", err)
	}
	return &result, nil
}

// ListModels returns the available models.
func (c *Client) ListModels() (*ModelList, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("thunderstt: create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thunderstt: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("thunderstt: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result ModelList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("thunderstt: decode response: %w", err)
	}
	return &result, nil
}

// Health checks if the server is alive.
func (c *Client) Health() (*HealthResponse, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("thunderstt: health check: %w", err)
	}
	defer resp.Body.Close()

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("thunderstt: decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}
