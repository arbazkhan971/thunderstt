package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thunderstt",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "thunderstt",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		},
		[]string{"method", "path"},
	)

	HTTPRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "thunderstt",
			Name:      "http_request_size_bytes",
			Help:      "HTTP request body size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(1024, 4, 8), // 1KB to ~64MB
		},
		[]string{"method", "path"},
	)

	// Transcription metrics
	TranscriptionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "thunderstt",
			Name:      "transcriptions_total",
			Help:      "Total number of transcription requests.",
		},
		[]string{"model", "status"}, // status: "success", "error"
	)

	TranscriptionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "thunderstt",
			Name:      "transcription_duration_seconds",
			Help:      "Time spent on transcription inference in seconds.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"model"},
	)

	AudioDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "thunderstt",
			Name:      "audio_duration_seconds",
			Help:      "Duration of input audio files in seconds.",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"model"},
	)

	// Queue metrics
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "thunderstt",
			Name:      "queue_depth",
			Help:      "Current number of jobs waiting or running in the queue.",
		},
	)

	QueueCapacity = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "thunderstt",
			Name:      "queue_capacity",
			Help:      "Maximum number of concurrent workers in the queue.",
		},
	)

	// Model metrics
	ModelLoaded = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thunderstt",
			Name:      "model_loaded",
			Help:      "Whether a model is loaded and ready (1) or not (0).",
		},
		[]string{"model"},
	)
)
