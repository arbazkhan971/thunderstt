package api

import (
	"net/http"
	"strings"
)

// BearerAuth returns middleware that requires a valid Bearer token in the
// Authorization header. If apiKey is empty, authentication is disabled and
// all requests pass through. Health and readiness endpoints are always exempt.
func BearerAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no key is configured.
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Exempt health/ready/metrics endpoints.
			path := r.URL.Path
			if path == "/health" || path == "/ready" || path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Check Authorization header.
			auth := r.Header.Get("Authorization")
			if auth == "" {
				WriteErrorWithCode(w, http.StatusUnauthorized, "missing_api_key", "missing Authorization header")
				return
			}

			// Accept "Bearer <token>" format.
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == auth { // no "Bearer " prefix found
				token = strings.TrimPrefix(auth, "bearer ")
			}
			if token == auth { // still no prefix
				WriteErrorWithCode(w, http.StatusUnauthorized, "invalid_auth_format", "Authorization header must use Bearer scheme")
				return
			}

			if strings.TrimSpace(token) != apiKey {
				WriteErrorWithCode(w, http.StatusUnauthorized, "invalid_api_key", "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
