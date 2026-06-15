package beamsync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestClientRateLimiterBlocksAfterLimit(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("first request should be allowed")
	}
	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("second request should be allowed")
	}
	if decision := limiter.allow("192.168.1.10"); decision.allowed || decision.retryAfter <= 0 {
		t.Fatalf("third request should be blocked with retryAfter, allowed=%v retryAfter=%s", decision.allowed, decision.retryAfter)
	}
}

func TestClientRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("first request should be allowed")
	}
	if decision := limiter.allow("192.168.1.10"); decision.allowed {
		t.Fatal("second request in the same window should be blocked")
	}

	now = now.Add(time.Minute)
	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestClientRateLimiterEvictsOldestClientWhenFull(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(2, time.Minute)
	limiter.maxClients = 2
	limiter.now = func() time.Time { return now }

	if decision := limiter.allow("192.168.1.10"); !decision.allowed {
		t.Fatal("first client should be allowed")
	}

	now = now.Add(time.Second)
	if decision := limiter.allow("192.168.1.11"); !decision.allowed {
		t.Fatal("second client should be allowed")
	}

	now = now.Add(time.Second)
	if decision := limiter.allow("192.168.1.12"); !decision.allowed {
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

	handler := rateLimitMiddleware(limiter, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.20:1234"

	first := httptest.NewRecorder()
	handler(first, req)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", first.Code, http.StatusNoContent)
	}
	if got := first.Header().Get("X-RateLimit-Limit"); got != "1" {
		t.Fatalf("X-RateLimit-Limit = %q, want %q", got, "1")
	}
	if got := first.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want %q", got, "0")
	}
	if got := first.Header().Get("X-RateLimit-Reset"); got != strconv.FormatInt(now.Add(time.Minute).Unix(), 10) {
		t.Fatalf("X-RateLimit-Reset = %q, want %q", got, strconv.FormatInt(now.Add(time.Minute).Unix(), 10))
	}

	second := httptest.NewRecorder()
	handler(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response should set Retry-After")
	}
	if got := second.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("rate-limited X-RateLimit-Remaining = %q, want %q", got, "0")
	}
}

func TestClientIPIgnoresSpoofedXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 192.168.1.100")

	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("clientIP = %q, want real RemoteAddr IP", got)
	}
}

func TestRateLimitMiddlewareIgnoresXForwardedFor(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	handler := rateLimitMiddleware(limiter, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.RemoteAddr = "10.0.0.1:1234"
	first.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	firstRecorder := httptest.NewRecorder()
	handler(firstRecorder, first)
	if firstRecorder.Code != http.StatusNoContent {
		t.Fatalf("first forwarded request status = %d, want %d", firstRecorder.Code, http.StatusNoContent)
	}

	secondSameForwardedIP := httptest.NewRequest(http.MethodGet, "/", nil)
	secondSameForwardedIP.RemoteAddr = "10.0.0.1:5678"
	secondSameForwardedIP.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	secondRecorder := httptest.NewRecorder()
	handler(secondRecorder, secondSameForwardedIP)
	if secondRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("same real client status = %d, want %d", secondRecorder.Code, http.StatusTooManyRequests)
	}

	otherForwardedIP := httptest.NewRequest(http.MethodGet, "/", nil)
	otherForwardedIP.RemoteAddr = "10.0.0.1:9999"
	otherForwardedIP.Header.Set("X-Forwarded-For", "203.0.113.11, 10.0.0.1")
	otherRecorder := httptest.NewRecorder()
	handler(otherRecorder, otherForwardedIP)
	if otherRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed forwarded client status = %d, want %d", otherRecorder.Code, http.StatusTooManyRequests)
	}
}

func TestTrustedDeviceChecksUseRemoteAddrInsteadOfForwardedFor(t *testing.T) {
	limiter := newClientRateLimiter(1, time.Minute)
	settings := &TransferSettings{
		TrustedDevices: []DeviceRule{{IP: "203.0.113.10", FriendlyName: "Spoofed phone"}},
	}

	var calls int
	handler := rateLimitMiddleware(limiter, settings, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	handler(httptest.NewRecorder(), req)
	handler(httptest.NewRecorder(), req)

	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1 because spoofed trusted IP must not bypass rate limiting", calls)
	}
}

func TestRateLimitMiddlewareBypassesTrustedDevice(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }
	settings := &TransferSettings{
		TrustedDevices: []DeviceRule{{IP: "192.168.1.50", FriendlyName: "Phone"}},
	}
	var calls int
	handler := rateLimitMiddleware(limiter, settings, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.50:1234"

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		handler(recorder, req)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("trusted request %d status = %d, want %d", i+1, recorder.Code, http.StatusNoContent)
		}
	}
	if calls != 3 {
		t.Fatalf("trusted device handler calls = %d, want %d", calls, 3)
	}
	if len(limiter.clients) != 0 {
		t.Fatal("trusted devices should bypass rate limiter accounting")
	}
}

func TestRateLimitMiddlewareRoundsRetryAfterUp(t *testing.T) {
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	limiter := newClientRateLimiter(1, 1600*time.Millisecond)
	limiter.now = func() time.Time { return now }

	handler := rateLimitMiddleware(limiter, nil, func(w http.ResponseWriter, r *http.Request) {
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
