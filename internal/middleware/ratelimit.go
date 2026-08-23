package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateBucket is a token bucket.
type rateBucket struct {
	tokens   float64
	capacity float64
	refill   float64 // tokens per second
	last     time.Time
}

func (b *rateBucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = minFloat(b.capacity, b.tokens+elapsed*b.refill)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// RateLimiter limits requests per key (client IP) using token buckets.
// Safe for concurrent use; stale buckets are reaped lazily.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*rateBucket
	capacity float64
	refill   float64 // tokens per second
	lastSweep time.Time
}

// NewRateLimiter allows burst of capacity, refilling refillPerSec tokens per
// second. Example: NewRateLimiter(30, 1) = bursts of 30, sustained 1 req/s.
func NewRateLimiter(capacity float64, refillPerSec float64) *RateLimiter {
	return &RateLimiter{
		buckets:   make(map[string]*rateBucket),
		capacity:  capacity,
		refill:    refillPerSec,
		lastSweep: time.Now(),
	}
}

// Allow reports whether a request from key may proceed now.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	if !exists {
		if len(rl.buckets) >= 10000 {
			rl.sweepLocked(now)
		}
		bucket = &rateBucket{tokens: rl.capacity, capacity: rl.capacity, refill: rl.refill, last: now}
		rl.buckets[key] = bucket
	}
	return bucket.allow(now)
}

// sweepLocked drops buckets idle for over an hour. Caller holds the lock.
func (rl *RateLimiter) sweepLocked(now time.Time) {
	for key, bucket := range rl.buckets {
		if now.Sub(bucket.last) > time.Hour {
			delete(rl.buckets, key)
		}
	}
	rl.lastSweep = now
}

// Middleware returns an HTTP middleware that answers 429 once the bucket
// for the client IP is exhausted.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientKey identifies the caller: X-Forwarded-For's first hop when present
// (set by the reverse proxy terminating TLS), otherwise the host part of
// RemoteAddr. The port is stripped — ephemeral ports would otherwise give
// every connection its own bucket.
func clientKey(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		for i := 0; i < len(forwarded); i++ {
			if forwarded[i] == ',' {
				return trimSpace(forwarded[:i])
			}
		}
		return trimSpace(forwarded)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // no port; use as-is
	}
	return host
}

func trimSpace(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}
