package server

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/d-kuro/kirocc/internal/httpx"
	"github.com/d-kuro/kirocc/internal/logging"
	"github.com/d-kuro/kirocc/internal/tracing"
)

func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := tracing.ExtractTraceID(r.Context())
		if traceID == "" {
			traceID = logging.NewTraceID()
		}
		ctx := logging.WithTraceID(r.Context(), traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authMiddleware enforces API key authentication when configured.
// Accepts Authorization: Bearer <key>, x-api-key, and anthropic-api-key headers.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		token := ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			if t, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
				token = t
			} else {
				token = authHeader
			}
		} else if xKey := r.Header.Get("x-api-key"); xKey != "" {
			token = xKey
		} else if aKey := r.Header.Get("anthropic-api-key"); aKey != "" {
			token = aKey
		}

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.apiKey)) != 1 {
			httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrTypeAuthentication, "invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers for localhost origins.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalhostOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, anthropic-api-key, X-Claude-Code-Session-Id")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLocalhostOrigin checks if the origin is a localhost URL using strict URL parsing.
func isLocalhostOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}
