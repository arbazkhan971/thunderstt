// Package config provides application configuration for ThunderSTT.
//
// Configuration values can be supplied through CLI flags (serve command),
// environment variables (THUNDERSTT_* prefix), or a combination of both.
// Flag values take precedence over environment variables; environment
// variables take precedence over built-in defaults.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Config holds all runtime configuration for ThunderSTT.
type Config struct {
	// Host is the address the HTTP server binds to.
	Host string

	// Port is the TCP port the HTTP server listens on.
	Port int

	// Model is the name of the whisper model to load (e.g. "base", "large-v3").
	Model string

	// Workers is the number of concurrent transcription workers.
	Workers int

	// LogLevel controls the minimum log severity (trace, debug, info, warn, error, fatal).
	LogLevel string

	// ModelsDir is the directory where downloaded model files are stored.
	ModelsDir string
}

// Defaults used when neither flags nor environment variables are set.
const (
	DefaultHost     = "0.0.0.0"
	DefaultPort     = 8080
	DefaultModel    = "base"
	DefaultLogLevel = "info"
)

// defaultModelsDir returns the platform-appropriate default models directory.
// It resolves to ~/.thunderstt/models.
func defaultModelsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".thunderstt", "models")
}

// NewFromServeFlags builds a Config from values that were parsed by cobra
// flags. Any zero-valued fields are back-filled from environment variables
// and then from hard-coded defaults.
func NewFromServeFlags(host string, port int, model string, workers int, logLevel string) *Config {
	cfg := &Config{
		Host:     host,
		Port:     port,
		Model:    model,
		Workers:  workers,
		LogLevel: logLevel,
	}

	// Back-fill from environment for fields the flags might not cover.
	cfg.ModelsDir = envOrDefault("THUNDERSTT_MODELS_DIR", defaultModelsDir())

	// Override zero / empty values from env.
	if cfg.Host == "" {
		cfg.Host = envOrDefault("THUNDERSTT_HOST", DefaultHost)
	}
	if cfg.Port == 0 {
		cfg.Port = envIntOrDefault("THUNDERSTT_PORT", DefaultPort)
	}
	if cfg.Model == "" {
		cfg.Model = envOrDefault("THUNDERSTT_MODEL", DefaultModel)
	}
	if cfg.Workers == 0 {
		cfg.Workers = envIntOrDefault("THUNDERSTT_WORKERS", runtime.NumCPU())
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = envOrDefault("THUNDERSTT_LOG_LEVEL", DefaultLogLevel)
	}

	return cfg
}

// NewFromEnv builds a Config purely from environment variables, falling
// back to defaults where an env var is unset. This is useful for sub-commands
// that do not accept the full set of serve flags.
func NewFromEnv() *Config {
	return &Config{
		Host:      envOrDefault("THUNDERSTT_HOST", DefaultHost),
		Port:      envIntOrDefault("THUNDERSTT_PORT", DefaultPort),
		Model:     envOrDefault("THUNDERSTT_MODEL", DefaultModel),
		Workers:   envIntOrDefault("THUNDERSTT_WORKERS", runtime.NumCPU()),
		LogLevel:  envOrDefault("THUNDERSTT_LOG_LEVEL", DefaultLogLevel),
		ModelsDir: envOrDefault("THUNDERSTT_MODELS_DIR", defaultModelsDir()),
	}
}

// Validate performs basic sanity checks on the configuration and returns an
// error describing the first problem found, or nil if everything looks good.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return &ValidationError{Field: "Port", Reason: "must be between 1 and 65535"}
	}
	if c.Workers < 1 {
		return &ValidationError{Field: "Workers", Reason: "must be at least 1"}
	}
	if c.Model == "" {
		return &ValidationError{Field: "Model", Reason: "must not be empty"}
	}
	return nil
}

// ValidationError describes a single configuration validation failure.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return "config: invalid " + e.Field + ": " + e.Reason
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// envOrDefault returns the value of the named environment variable, or
// fallback if the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envIntOrDefault returns the integer value of the named environment variable,
// or fallback if the variable is unset, empty, or not a valid integer.
func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
