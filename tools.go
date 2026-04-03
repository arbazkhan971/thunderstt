//go:build tools

// Package thunderstt pins dependencies that are used by other packages in
// this module but not yet imported in the initial scaffold. This file ensures
// `go mod tidy` does not prune them.
package thunderstt

import (
	_ "github.com/go-chi/chi/v5"
	_ "github.com/prometheus/client_golang/prometheus"
)
