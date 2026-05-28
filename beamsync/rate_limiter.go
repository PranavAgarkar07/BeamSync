package beamsync

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type clientRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
	now      func() time.Time
}

func newClientRateLimiter(limit int, window time.Duration) *clientRateLimiter {
	return &clientRateLimiter{
		limit:    limit,
		window:   window,
		requests: make(map[string][]time.Time),
		now:      time.Now,
	}
}

func (l *clientRateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	seen := l.requests[key]
	keepFrom := 0
	for keepFrom < len(seen) && seen[keepFrom].Before(cutoff) {
		keepFrom++
	}
	seen = seen[keepFrom:]

	if len(seen) >= l.limit {
		l.requests[key] = seen
		return false
	}

	l.requests[key] = append(seen, now)
	return true
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func rateLimitMiddleware(limiter *clientRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if !limiter.allow(clientAddress(r)) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "429 Too Many Requests: rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
