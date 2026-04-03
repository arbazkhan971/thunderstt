// Package api implements the HTTP API layer for ThunderSTT, exposing
// OpenAI-compatible transcription endpoints over REST.
package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/metrics"
	"github.com/arbaz/thunderstt/internal/pipeline"
	"github.com/arbaz/thunderstt/internal/queue"
)

// ServerConfig extends the base config with HTTP-specific settings that
// may be set at the API layer.
type ServerConfig struct {
	// Embed the application config.
	*config.Config

	// MaxFileSize is the maximum upload file size in bytes.
	// Zero means use the default (25 MB).
	MaxFileSize int64

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum time to wait for the next request on
	// a keep-alive connection.
	IdleTimeout time.Duration

	// ShutdownTimeout is the maximum time to wait for in-flight requests
	// to complete during graceful shutdown.
	ShutdownTimeout time.Duration

	// RequestTimeout is the maximum time a single request may take
	// end-to-end (including transcription).
	RequestTimeout time.Duration
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig(cfg *config.Config) *ServerConfig {
	return &ServerConfig{
		Config:          cfg,
		MaxFileSize:     25 * 1024 * 1024, // 25 MB
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    5 * time.Minute, // transcription can take a while
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		RequestTimeout:  5 * time.Minute,
	}
}

// Server is the HTTP server for ThunderSTT. It holds references to the
// transcription pipeline, job queue, configuration, and the chi router.
type Server struct {
	router   chi.Router
	pipeline *pipeline.Pipeline
	cfg      *ServerConfig
	queue    *queue.Queue
}

// NewServer creates a fully configured Server with all middleware and routes
// wired up. The pipeline handles audio-to-text processing; cfg controls
// server behaviour and limits.
func NewServer(p *pipeline.Pipeline, cfg *config.Config) *Server {
	scfg := DefaultServerConfig(cfg)

	s := &Server{
		router:   chi.NewRouter(),
		pipeline: p,
		cfg:      scfg,
		queue:    queue.NewQueue(cfg.Workers),
	}

	// Record queue capacity metric.
	metrics.QueueCapacity.Set(float64(cfg.Workers))

	// Global middleware stack (order matters).
	s.router.Use(RequestID)
	s.router.Use(Logging)
	s.router.Use(Recovery)
	s.router.Use(CORS)
	s.router.Use(MaxBodySize(scfg.MaxFileSize))
	s.router.Use(RequestTimeout(scfg.RequestTimeout))

	// Register routes.
	SetupRoutes(s.router, s)

	return s
}

// Start begins listening for HTTP requests and blocks until a termination
// signal (SIGINT, SIGTERM) is received. It then initiates graceful shutdown,
// allowing in-flight requests to complete within the configured timeout.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  s.cfg.IdleTimeout,
	}

	// Channel to capture server errors.
	errCh := make(chan error, 1)

	// Start the server in a goroutine.
	go func() {
		log.Info().
			Str("addr", addr).
			Msg("HTTP server listening")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info().
			Str("signal", sig.String()).
			Msg("shutdown signal received, draining connections...")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	// Graceful shutdown with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Info().Msg("HTTP server stopped gracefully")
	return nil
}

// Router returns the underlying chi.Router, useful for testing.
func (s *Server) Router() chi.Router {
	return s.router
}
