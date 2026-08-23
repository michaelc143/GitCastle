// Package middleware provides composable HTTP middleware for production
// hardening: security headers, request IDs, panic recovery, and rate limiting.
package middleware

import (
	"net/http"
)

// Chain applies middleware left to right (first listed runs first).
func Chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// securityHeaders are set on every response.
var securityHeaders = map[string]string{
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Referrer-Policy":        "strict-origin-when-cross-origin",
	// API + SPA served from the same origin; allow self, inline styles for
	// the bundled CSS-in-HTML, and no legacy plugins.
	"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:; script-src 'self'",
}

// SecureHeaders sets standard hardening headers on every response.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range securityHeaders {
			w.Header().Set(key, value)
		}
		next.ServeHTTP(w, r)
	})
}
