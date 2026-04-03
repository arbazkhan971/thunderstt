package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// errorResponse is the JSON envelope for error responses.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// WriteJSON serialises data as JSON and writes it to the response with the
// given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// WriteErrorWithCode writes a structured error response with an explicit error code.
func WriteErrorWithCode(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := errorResponse{
		Error: errorDetail{
			Message: message,
			Type:    httpStatusToErrorType(status),
			Code:    code,
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("failed to encode error response")
	}
}

// WriteError writes a structured error response in the OpenAI error format.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := errorResponse{
		Error: errorDetail{
			Message: message,
			Type:    httpStatusToErrorType(status),
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("failed to encode error response")
	}
}

// WriteBytes writes raw bytes with the specified content type and status.
func WriteBytes(w http.ResponseWriter, status int, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if _, err := w.Write(data); err != nil {
		log.Error().Err(err).Msg("failed to write response bytes")
	}
}

// httpStatusToErrorType maps HTTP status codes to OpenAI-style error type strings.
func httpStatusToErrorType(status int) string {
	switch {
	case status == http.StatusBadRequest:
		return "invalid_request_error"
	case status == http.StatusUnauthorized:
		return "authentication_error"
	case status == http.StatusForbidden:
		return "permission_error"
	case status == http.StatusNotFound:
		return "not_found_error"
	case status == http.StatusRequestEntityTooLarge:
		return "invalid_request_error"
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status >= 500:
		return "server_error"
	default:
		return "api_error"
	}
}
