package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSecureHeadersSetOnResponse(t *testing.T) {
	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, header := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Content-Security-Policy"} {
		if recorder.Header().Get(header) == "" {
			t.Fatalf("missing security header %s", header)
		}
	}
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" || len(seen) < 8 {
		t.Fatalf("request id = %q", seen)
	}
	if echoed := recorder.Header().Get("X-Request-ID"); echoed != seen {
		t.Fatalf("echoed %q != context %q", echoed, seen)
	}
}

func TestRequestIDHonorsIncomingHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "my-trace-id-1234")
	recorder := httptest.NewRecorder()
	RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).ServeHTTP(recorder, request)
	if got := recorder.Header().Get("X-Request-ID"); got != "my-trace-id-1234" {
		t.Fatalf("incoming ID not honored: %q", got)
	}
}

func TestRecoveryConvertsPanicTo500(t *testing.T) {
	logger := discardLogger()
	handler := Chain(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("boom")
		}),
		RequestID,
		Recovery(logger),
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Error     string `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Error != "internal server error" || body.RequestID == "" {
		t.Fatalf("body = %+v", body)
	}
}

func TestRateLimiterBurstThenReject(t *testing.T) {
	limiter := NewRateLimiter(3, 0.001)
	key := "1.2.3.4:5678"

	for i := 0; i < 3; i++ {
		if !limiter.Allow(key) {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
	if limiter.Allow(key) {
		t.Fatal("fourth immediate request should be rejected")
	}
	// A different key is unaffected.
	if !limiter.Allow("5.6.7.8:1234") {
		t.Fatal("independent key should be allowed")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter := NewRateLimiter(1, 1000) // refills 1000/sec
	if !limiter.Allow("k") {
		t.Fatal("first request allowed")
	}
	// Wait for at least one token to refill.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if limiter.Allow("k") {
			return // refilled and consumed
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("token never refilled")
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(1000, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Allow("shared-key")
		}()
	}
	wg.Wait() // race detector will flag unsynchronized access
}

func TestClientKeyPrefersForwardedFor(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.1:9999"
	if got := clientKey(request); got != "10.0.0.1" {
		t.Fatalf("host key = %q, want port stripped", got)
	}
	request.RemoteAddr = "no-port-value" // malformed; used verbatim
	if got := clientKey(request); got != "no-port-value" {
		t.Fatalf("malformed addr key = %q", got)
	}
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")
	if got := clientKey(request); got != "203.0.113.7" {
		t.Fatalf("forwarded key = %q", got)
	}
}

func TestChainOrderLeftToRight(t *testing.T) {
	var order []string
	mk := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+"-in")
				next.ServeHTTP(w, r)
				order = append(order, name+"-out")
			})
		}
	}
	Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), mk("first"), mk("second")).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if len(order) != 4 || order[0] != "first-in" || order[1] != "second-in" ||
		order[2] != "second-out" || order[3] != "first-out" {
		t.Fatalf("chain order = %v", order)
	}
}
