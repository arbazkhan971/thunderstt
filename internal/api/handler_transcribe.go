package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/format"
	"github.com/arbaz/thunderstt/internal/metrics"
)

// HandleTranscribe processes POST /v1/audio/transcriptions requests.
// It parses the multipart form, validates inputs, saves the uploaded file
// to a temporary directory, runs the transcription pipeline, formats the
// result, and returns it with the appropriate content type.
func (s *Server) HandleTranscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := reqLogger(ctx)

	// Parse multipart form with the configured max body size.
	maxBytes := s.maxFileSize()
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			WriteErrorWithCode(w, http.StatusRequestEntityTooLarge, "file_too_large",
				"file size exceeds the maximum allowed size")
			return
		}
		logger.Warn().Err(err).Msg("failed to parse multipart form")
		WriteErrorWithCode(w, http.StatusBadRequest, "invalid_request", "failed to parse multipart form: "+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	// Parse and validate the request fields.
	req, err := ParseTranscribeRequest(r)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid transcription request")
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer req.File.Close()

	logger.Info().
		Str("filename", req.FileHeader.Filename).
		Int64("size", req.FileHeader.Size).
		Str("model", req.Model).
		Str("language", req.Language).
		Str("response_format", req.ResponseFormat).
		Msg("transcription request received")

	// Validate the pipeline is ready.
	if s.pipeline == nil || !s.pipeline.Ready() {
		WriteErrorWithCode(w, http.StatusServiceUnavailable, "model_not_ready", "model is not loaded; server is not ready")
		return
	}

	// Save the uploaded file to a temporary location for processing.
	tmpPath, err := saveUploadedFile(req)
	if err != nil {
		logger.Error().Err(err).Msg("failed to save uploaded file")
		WriteError(w, http.StatusInternalServerError, "failed to process uploaded file")
		return
	}
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warn().Err(removeErr).Str("path", tmpPath).Msg("failed to remove temp file")
		}
	}()

	// Build engine options from the request.
	opts := engine.Options{
		Language:       req.Language,
		WordTimestamps: req.WantsWordTimestamps(),
		VADFilter:      true,
	}

	// Submit transcription to the queue for bounded concurrency.
	transcribeStart := time.Now()
	result, err := s.queue.Submit(ctx, func() (*engine.Result, error) {
		return s.pipeline.TranscribeFile(ctx, tmpPath, opts)
	})
	if err != nil {
		// Distinguish between context cancellation and internal errors.
		if ctx.Err() != nil {
			metrics.TranscriptionTotal.WithLabelValues(req.Model, "error").Inc()
			logger.Warn().Err(err).Msg("transcription cancelled")
			WriteErrorWithCode(w, http.StatusServiceUnavailable, "timeout", "request cancelled or timed out")
			return
		}
		metrics.TranscriptionTotal.WithLabelValues(req.Model, "error").Inc()
		logger.Error().Err(err).Msg("transcription failed")
		WriteErrorWithCode(w, http.StatusInternalServerError, "transcription_error", "transcription failed: "+err.Error())
		return
	}

	transcribeDuration := time.Since(transcribeStart)
	metrics.TranscriptionTotal.WithLabelValues(req.Model, "success").Inc()
	metrics.TranscriptionDuration.WithLabelValues(req.Model).Observe(transcribeDuration.Seconds())
	if result != nil {
		metrics.AudioDuration.WithLabelValues(req.Model).Observe(result.Duration)
	}

	// Format the result according to the requested response_format.
	data, contentType, err := format.FormatResult(result, req.ResponseFormat)
	if err != nil {
		logger.Error().Err(err).Msg("failed to format transcription result")
		WriteError(w, http.StatusInternalServerError, "failed to format response")
		return
	}

	logger.Info().
		Str("format", req.ResponseFormat).
		Int("result_bytes", len(data)).
		Float64("duration_secs", result.Duration).
		Msg("transcription completed")

	WriteBytes(w, http.StatusOK, contentType, data)
}

// saveUploadedFile writes the multipart file to a temp file on disk and
// returns the path. The caller is responsible for removing the file.
func saveUploadedFile(req *TranscribeRequest) (string, error) {
	// Determine a file extension from the original filename for codec hinting.
	ext := filepath.Ext(req.FileHeader.Filename)
	if ext == "" {
		ext = ".bin"
	}

	tmpFile, err := os.CreateTemp("", "thunderstt-upload-*"+ext)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmpFile, req.File); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// maxFileSize returns the maximum allowed upload size in bytes.
// Defaults to 25 MB. In the future this can be read from config.
func (s *Server) maxFileSize() int64 {
	const defaultMaxFileSize = 25 * 1024 * 1024 // 25 MB
	if s.cfg != nil && s.cfg.MaxFileSize > 0 {
		return s.cfg.MaxFileSize
	}
	return defaultMaxFileSize
}
