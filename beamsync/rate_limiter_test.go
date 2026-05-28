package beamsync

import (
	"encoding/json"
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

func TestClientRateLimiterEvictsOldestClientWhenFull(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute)
	limiter.maxClients = 2
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.allow("192.168.1.10"); !allowed {
		t.Fatal("first client should be allowed")
	}

	now = now.Add(time.Second)
	if allowed, _ := limiter.allow("192.168.1.11"); !allowed {
		t.Fatal("second client should be allowed")
	}

	now = now.Add(time.Second)
	if allowed, _ := limiter.allow("192.168.1.12"); !allowed {
		t.Fatal("new client should be admitted after oldest eviction")
	}

	if limiter.clients["192.168.1.10"] != nil {
		t.Fatal("oldest client should be evicted when the limiter is full")
	}
	if limiter.clients["192.168.1.11"] == nil || limiter.clients["192.168.1.12"] == nil {
		t.Fatal("newest clients should remain after eviction")
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

func TestRateLimitMiddlewareRoundsRetryAfterUp(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, 1600*time.Millisecond)
	limiter.now = func() time.Time { return now }

	handler := rateLimitMiddleware(limiter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.21:1234"

	handler(httptest.NewRecorder(), req)

	now = now.Add(500 * time.Millisecond)
	second := httptest.NewRecorder()
	handler(second, req)

	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if got := second.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want %q", got, "2")
	}

	var body struct {
		Error      string `json:"error"`
		Message    string `json:"message"`
		RetryAfter int64  `json:"retryAfter"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error != "rate_limit_exceeded" {
		t.Fatalf("error = %q, want %q", body.Error, "rate_limit_exceeded")
	}
	if body.Message != "429 Too Many Requests: rate limit exceeded" {
		t.Fatalf("message = %q, want %q", body.Message, "429 Too Many Requests: rate limit exceeded")
	}
	if body.RetryAfter != 2 {
		t.Fatalf("retryAfter = %d, want %d", body.RetryAfter, 2)
	}
}
