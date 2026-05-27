package beamsync

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRateLimiterBlocksAfterLimit(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.allow("192.168.1.10"); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.allow("192.168.1.10"); !allowed {
		t.Fatal("second request should be allowed")
	}
	if allowed, retryAfter := limiter.allow("192.168.1.10"); allowed || retryAfter <= 0 {
		t.Fatalf("third request should be blocked with retryAfter, allowed=%v retryAfter=%s", allowed, retryAfter)
	}
}

func TestClientRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.allow("192.168.1.10"); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.allow("192.168.1.10"); allowed {
		t.Fatal("second request in the same window should be blocked")
	}

	now = now.Add(time.Minute)
	if allowed, _ := limiter.allow("192.168.1.10"); !allowed {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	handler := rateLimitMiddleware(limiter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.20:1234"

	first := httptest.NewRecorder()
	handler(first, req)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	handler(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response should set Retry-After")
	}
}
