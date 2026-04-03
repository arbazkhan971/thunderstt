//go:build !cgo

package config

import (
	"os"
	"runtime"
	"testing"
)

func TestNewFromEnv_Defaults(t *testing.T) {
	// Clear all relevant env vars to test defaults.
	envVars := []string{
		"THUNDERSTT_HOST",
		"THUNDERSTT_PORT",
		"THUNDERSTT_MODEL",
		"THUNDERSTT_WORKERS",
		"THUNDERSTT_LOG_LEVEL",
		"THUNDERSTT_MODELS_DIR",
	}
	for _, key := range envVars {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg := NewFromEnv()

	if cfg.Host != DefaultHost {
		t.Errorf("Host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Workers != runtime.NumCPU() {
		t.Errorf("Workers = %d, want %d (NumCPU)", cfg.Workers, runtime.NumCPU())
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	// ModelsDir should be set to something non-empty (defaultModelsDir).
	if cfg.ModelsDir == "" {
		t.Error("ModelsDir should not be empty")
	}
}

func TestNewFromEnv_WithEnvVars(t *testing.T) {
	t.Setenv("THUNDERSTT_HOST", "127.0.0.1")
	t.Setenv("THUNDERSTT_PORT", "9090")
	t.Setenv("THUNDERSTT_MODEL", "large-v3")
	t.Setenv("THUNDERSTT_WORKERS", "4")
	t.Setenv("THUNDERSTT_LOG_LEVEL", "debug")
	t.Setenv("THUNDERSTT_MODELS_DIR", "/tmp/models")

	cfg := NewFromEnv()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.Model != "large-v3" {
		t.Errorf("Model = %q, want %q", cfg.Model, "large-v3")
	}
	if cfg.Workers != 4 {
		t.Errorf("Workers = %d, want 4", cfg.Workers)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.ModelsDir != "/tmp/models" {
		t.Errorf("ModelsDir = %q, want %q", cfg.ModelsDir, "/tmp/models")
	}
}

func TestNewFromEnv_InvalidPort(t *testing.T) {
	t.Setenv("THUNDERSTT_PORT", "not-a-number")

	cfg := NewFromEnv()

	// Should fall back to default when env var is not a valid integer.
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want default %d for invalid env", cfg.Port, DefaultPort)
	}
}

func TestNewFromServeFlags_Overrides(t *testing.T) {
	// Ensure env vars do not interfere.
	t.Setenv("THUNDERSTT_HOST", "env-host")
	t.Setenv("THUNDERSTT_PORT", "9999")
	t.Setenv("THUNDERSTT_MODEL", "env-model")

	cfg := NewFromServeFlags("flag-host", 7777, "flag-model", 2, "warn")

	// Flags should take precedence over env vars.
	if cfg.Host != "flag-host" {
		t.Errorf("Host = %q, want %q", cfg.Host, "flag-host")
	}
	if cfg.Port != 7777 {
		t.Errorf("Port = %d, want 7777", cfg.Port)
	}
	if cfg.Model != "flag-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "flag-model")
	}
	if cfg.Workers != 2 {
		t.Errorf("Workers = %d, want 2", cfg.Workers)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}
}

func TestNewFromServeFlags_ZeroValuesFallToEnv(t *testing.T) {
	t.Setenv("THUNDERSTT_HOST", "env-host")
	t.Setenv("THUNDERSTT_PORT", "9999")
	t.Setenv("THUNDERSTT_MODEL", "env-model")
	t.Setenv("THUNDERSTT_WORKERS", "8")
	t.Setenv("THUNDERSTT_LOG_LEVEL", "trace")

	// Pass zero/empty values for all flags.
	cfg := NewFromServeFlags("", 0, "", 0, "")

	if cfg.Host != "env-host" {
		t.Errorf("Host = %q, want %q (from env)", cfg.Host, "env-host")
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999 (from env)", cfg.Port)
	}
	if cfg.Model != "env-model" {
		t.Errorf("Model = %q, want %q (from env)", cfg.Model, "env-model")
	}
	if cfg.Workers != 8 {
		t.Errorf("Workers = %d, want 8 (from env)", cfg.Workers)
	}
	if cfg.LogLevel != "trace" {
		t.Errorf("LogLevel = %q, want %q (from env)", cfg.LogLevel, "trace")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Host:    "0.0.0.0",
		Port:    8080,
		Model:   "base",
		Workers: 2,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() returned error for valid config: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Port: tt.port, Model: "base", Workers: 1}
			err := cfg.Validate()
			if err == nil {
				t.Error("expected validation error for invalid port")
			}
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("expected *ValidationError, got %T", err)
			}
			if ve.Field != "Port" {
				t.Errorf("error field = %q, want %q", ve.Field, "Port")
			}
		})
	}
}

func TestValidate_InvalidWorkers(t *testing.T) {
	cfg := &Config{Port: 8080, Model: "base", Workers: 0}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for Workers=0")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Field != "Workers" {
		t.Errorf("error field = %q, want %q", ve.Field, "Workers")
	}
}

func TestValidate_EmptyModel(t *testing.T) {
	cfg := &Config{Port: 8080, Model: "", Workers: 1}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for empty Model")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Field != "Model" {
		t.Errorf("error field = %q, want %q", ve.Field, "Model")
	}
}

func TestValidate_BoundaryPorts(t *testing.T) {
	// Port 1 and 65535 should be valid.
	cfg := &Config{Port: 1, Model: "base", Workers: 1}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Port=1 should be valid, got: %v", err)
	}

	cfg.Port = 65535
	if err := cfg.Validate(); err != nil {
		t.Errorf("Port=65535 should be valid, got: %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "Port", Reason: "must be between 1 and 65535"}
	msg := ve.Error()
	if msg != "config: invalid Port: must be between 1 and 65535" {
		t.Errorf("error message = %q, unexpected format", msg)
	}
}
