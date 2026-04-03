package model

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const (
	// defaultCacheSubdir is the path appended to the user's home directory
	// when no explicit models directory is configured.
	defaultCacheSubdir = ".cache/thunderstt/models"

	// envModelsDir is the environment variable that overrides the default
	// model cache location.
	envModelsDir = "THUNDERSTT_MODELS_DIR"
)

// DefaultCacheDir returns the default directory used to store downloaded
// model files. If the THUNDERSTT_MODELS_DIR environment variable is set,
// its value is returned. Otherwise the default is ~/.cache/thunderstt/models.
func DefaultCacheDir() string {
	if dir := os.Getenv(envModelsDir); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to current directory if we cannot determine home.
		return filepath.Join(".", defaultCacheSubdir)
	}
	return filepath.Join(home, defaultCacheSubdir)
}

// ModelPath returns the filesystem path where the given model's files would
// be stored (or are stored) in the cache. It does not verify that the path
// exists or that the model is fully downloaded.
func ModelPath(modelID string) string {
	return filepath.Join(DefaultCacheDir(), modelID)
}

// IsModelCached checks whether all required files for the given model are
// present in the cache directory. It returns false if the model is unknown
// or if any required file is missing.
func IsModelCached(modelID string) bool {
	info, err := GetModel(modelID)
	if err != nil {
		return false
	}

	dir := ModelPath(modelID)

	for _, file := range info.Files {
		path := filepath.Join(dir, file)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return false
		}
		// A zero-length file is considered incomplete / corrupted.
		if st.Size() == 0 {
			return false
		}
	}
	return true
}

// EnsureModel guarantees that the model identified by modelID is available
// on disk, downloading it from HuggingFace Hub if necessary. It returns the
// path to the model directory on success.
func EnsureModel(modelID string) (string, error) {
	// Validate that the model is known before doing any filesystem work.
	if _, err := GetModel(modelID); err != nil {
		return "", err
	}

	dir := ModelPath(modelID)

	if IsModelCached(modelID) {
		log.Info().
			Str("model", modelID).
			Str("path", dir).
			Msg("model already cached")
		return dir, nil
	}

	log.Info().
		Str("model", modelID).
		Str("path", dir).
		Msg("downloading model")

	if err := DownloadModel(modelID, dir); err != nil {
		return "", fmt.Errorf("model: ensure %s: %w", modelID, err)
	}

	// Verify the download produced all expected files.
	if !IsModelCached(modelID) {
		return "", fmt.Errorf("model: download of %s completed but required files are missing", modelID)
	}

	log.Info().
		Str("model", modelID).
		Str("path", dir).
		Msg("model ready")

	return dir, nil
}
