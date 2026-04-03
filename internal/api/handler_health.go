package api

import (
	"net/http"
	"runtime"
	"time"
)

// serverStartTime tracks when the server was initialized.
var serverStartTime = time.Now()

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
}

// readyResponse is the JSON body returned by the readiness endpoint.
type readyResponse struct {
	Status     string      `json:"status"`
	Model      string      `json:"model,omitempty"`
	QueueDepth int         `json:"queue_depth,omitempty"`
	QueueMax   int         `json:"queue_capacity,omitempty"`
	SystemInfo *systemInfo `json:"system,omitempty"`
}

// systemInfo holds runtime diagnostics included in the readiness response.
type systemInfo struct {
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAllocMB   int    `json:"mem_alloc_mb"`
}

// HandleHealth returns 200 OK unconditionally. It signals that the process
// is running and the HTTP stack is functional (liveness probe).
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Uptime: time.Since(serverStartTime).Round(time.Second).String(),
	})
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

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	WriteJSON(w, http.StatusOK, readyResponse{
		Status:     "ready",
		Model:      s.pipeline.ModelName(),
		QueueDepth: s.queue.Len(),
		QueueMax:   s.queue.Cap(),
		SystemInfo: &systemInfo{
			GoVersion:    runtime.Version(),
			NumCPU:       runtime.NumCPU(),
			NumGoroutine: runtime.NumGoroutine(),
			MemAllocMB:   int(memStats.Alloc / 1024 / 1024),
		},
	})
}
