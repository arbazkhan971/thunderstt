package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes registers all HTTP routes on the given chi router. The routes
// follow the OpenAI API conventions for audio transcription.
func SetupRoutes(r chi.Router, s *Server) {
	// Health and readiness probes (unversioned, no auth).
	r.Get("/health", s.HandleHealth)
	r.Get("/ready", s.HandleReady)
	r.Get("/version", s.HandleVersion)

	// Prometheus metrics endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// API v1 routes.
	r.Route("/v1", func(v1 chi.Router) {
		// Transcription endpoint (OpenAI-compatible).
		v1.Post("/audio/transcriptions", s.HandleTranscribe)

		// Translation endpoint (OpenAI-compatible).
		v1.Post("/audio/translations", s.HandleTranslate)

		// Model listing (OpenAI-compatible).
		v1.Get("/models", s.HandleListModels)

		// WebSocket streaming endpoint.
		v1.Get("/audio/stream", s.HandleStream)
	})
}
