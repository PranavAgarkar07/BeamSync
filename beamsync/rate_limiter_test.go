package beamsync

import (
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
