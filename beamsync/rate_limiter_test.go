package beamsync

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRateLimiterBlocksAfterLimit(t *testing.T) {
	limiter := newClientRateLimiter(2, time.Minute)
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("192.168.1.10") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.allow("192.168.1.10") {
		t.Fatal("second request should be allowed")
	}
	if limiter.allow("192.168.1.10") {
		t.Fatal("third request should be blocked")
	}
}

func TestClientRateLimiterAllowsAfterWindow(t *testing.T) {
	limiter := newClientRateLimiter(1, time.Minute)
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("192.168.1.10") {
		t.Fatal("first request should be allowed")
	}

	now = now.Add(time.Minute + time.Second)
	if !limiter.allow("192.168.1.10") {
		t.Fatal("request after window should be allowed")
	}
}

func TestRateLimitMiddlewareReturnsTooManyRequests(t *testing.T) {
	limiter := newClientRateLimiter(1, time.Minute)
	handler := rateLimitMiddleware(limiter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	for i, want := range []int{http.StatusAccepted, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.RemoteAddr = "192.168.1.10:5000"
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d", i+1, rec.Code, want)
		}
	}
}
