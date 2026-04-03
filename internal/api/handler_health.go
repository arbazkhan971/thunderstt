package api

import (
	"net/http"
)

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status string `json:"status"`
}

// readyResponse is the JSON body returned by the readiness endpoint.
type readyResponse struct {
	Status string `json:"status"`
	Model  string `json:"model,omitempty"`
}

// HandleHealth returns 200 OK unconditionally. It signals that the process
// is running and the HTTP stack is functional (liveness probe).
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// HandleReady returns 200 OK if the pipeline has loaded its model and is
// ready to accept transcription requests. Otherwise it returns 503 Service
// Unavailable. This is intended for use as a Kubernetes readiness probe.
func (s *Server) HandleReady(w http.ResponseWriter, r *http.Request) {
	if s.pipeline == nil || !s.pipeline.Ready() {
		WriteJSON(w, http.StatusServiceUnavailable, readyResponse{
			Status: "not_ready",
		})
		return
	}

	WriteJSON(w, http.StatusOK, readyResponse{
		Status: "ready",
		Model:  s.pipeline.ModelName(),
	})
}
