package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/format"
	"github.com/arbaz/thunderstt/internal/metrics"
)

// HandleTranslate processes POST /v1/audio/translations requests.
// It translates audio to English text, matching the OpenAI translations API.
func (s *Server) HandleTranslate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := reqLogger(ctx)

	maxBytes := s.maxFileSize()
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			WriteErrorWithRequest(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "file size exceeds the maximum allowed size")
			return
		}
		WriteErrorWithRequest(w, r, http.StatusBadRequest, "invalid_request", "failed to parse multipart form: "+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	// Parse the request (reuse transcription parser).
	req, err := ParseTranscribeRequest(r)
	if err != nil {
		WriteErrorWithRequest(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	defer req.File.Close()

	// Validate audio filename.
	if err := ValidateAudioFilename(req.FileHeader.Filename); err != nil {
		WriteErrorWithRequest(w, r, http.StatusBadRequest, "invalid_file_type", err.Error())
		return
	}

	logger.Info().
		Str("filename", req.FileHeader.Filename).
		Int64("size", req.FileHeader.Size).
		Str("model", req.Model).
		Str("response_format", req.ResponseFormat).
		Msg("translation request received")

	if s.pipeline == nil || !s.pipeline.Ready() {
		WriteErrorWithRequest(w, r, http.StatusServiceUnavailable, "model_not_ready", "model is not loaded; server is not ready")
		return
	}

	tmpPath, err := saveUploadedFile(req)
	if err != nil {
		logger.Error().Err(err).Msg("failed to save uploaded file")
		WriteErrorWithRequest(w, r, http.StatusInternalServerError, "internal_error", "failed to process uploaded file")
		return
	}
	defer os.Remove(tmpPath)

	// Translation always targets English.
	opts := engine.Options{
		Language:       "en",
		WordTimestamps: req.WantsWordTimestamps(),
		VADFilter:      true,
	}

	transcribeStart := time.Now()

	result, err := s.queue.Submit(ctx, func() (*engine.Result, error) {
		return s.pipeline.TranscribeFile(ctx, tmpPath, opts)
	})
	if err != nil {
		if ctx.Err() != nil {
			metrics.TranscriptionTotal.WithLabelValues(req.Model, "error").Inc()
			WriteErrorWithRequest(w, r, http.StatusServiceUnavailable, "timeout", "request cancelled or timed out")
			return
		}
		metrics.TranscriptionTotal.WithLabelValues(req.Model, "error").Inc()
		WriteErrorWithRequest(w, r, http.StatusInternalServerError, "transcription_error", "translation failed: "+err.Error())
		return
	}

	transcribeDuration := time.Since(transcribeStart)
	metrics.TranscriptionTotal.WithLabelValues(req.Model, "success").Inc()
	metrics.TranscriptionDuration.WithLabelValues(req.Model).Observe(transcribeDuration.Seconds())
	if result != nil {
		metrics.AudioDuration.WithLabelValues(req.Model).Observe(result.Duration)
	}

	// Force English in the result.
	if result != nil {
		result.Language = "en"
	}

	data, contentType, err := format.FormatResult(result, req.ResponseFormat)
	if err != nil {
		WriteErrorWithRequest(w, r, http.StatusInternalServerError, "format_error", "failed to format response")
		return
	}

	logger.Info().
		Str("format", req.ResponseFormat).
		Int("result_bytes", len(data)).
		Msg("translation completed")

	WriteBytes(w, http.StatusOK, contentType, data)
}
