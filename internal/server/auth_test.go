package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
)

func TestAuthMiddleware(t *testing.T) {
	// Setup test cases
	tests := []struct {
		name           string
		configToken    string
		requestToken   string
		expectedStatus int
	}{
		{
			name:           "no token configured",
			configToken:    "",
			requestToken:   "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid token",
			configToken:    "test-token",
			requestToken:   "test-token",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid token with bearer prefix",
			configToken:    "test-token",
			requestToken:   "Bearer test-token",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid token",
			configToken:    "test-token",
			requestToken:   "wrong-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing token",
			configToken:    "test-token",
			requestToken:   "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create server with configuration
			cfg := &config.Config{}
			cfg.Server.Auth.APIToken = tc.configToken

			srv := &Server{
				cfg: cfg,
			}

			// Create a test handler that will be wrapped by the middleware
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tc.requestToken != "" {
				req.Header.Set("Authorization", tc.requestToken)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Apply middleware to test handler and call it
			srv.authMiddleware(testHandler).ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}
