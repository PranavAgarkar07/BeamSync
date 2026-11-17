package beamsync

import (
	"sync"
	"time"
)

type rateLimitState struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type rateLimitDecision struct {
	allowed    bool
	retryAfter time.Duration
	limit      int
	remaining  int
	resetAt    time.Time
}

type clientRateLimiter struct {
	mu         sync.Mutex
	limit      int
	window     time.Duration
	maxClients int
	clients    map[string]*rateLimitState
	now        func() time.Time
}

func newClientRateLimiter(limit int, window time.Duration) *clientRateLimiter {
	return &clientRateLimiter{
		limit:      limit,
		window:     window,
		maxClients: 4096,
		clients:    make(map[string]*rateLimitState),
		now:        time.Now,
	}
}

func (l *clientRateLimiter) allow(client string) rateLimitDecision {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return rateLimitDecision{allowed: true}
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.prune(now)

	state, ok := l.clients[client]
	if !ok || now.Sub(state.windowStart) >= l.window {
		if !ok {
			l.evictOldestIfFull()
		}
		l.clients[client] = &rateLimitState{
			windowStart: now,
			count:       1,
			lastSeen:    now,
		}
		return rateLimitDecision{
			allowed:   true,
			limit:     l.limit,
			remaining: l.limit - 1,
			resetAt:   now.Add(l.window),
		}
	}

	state.lastSeen = now
	resetAt := state.windowStart.Add(l.window)
	if state.count >= l.limit {
		return rateLimitDecision{
			allowed:    false,
			retryAfter: l.window - now.Sub(state.windowStart),
			limit:      l.limit,
			remaining:  0,
			resetAt:    resetAt,
		}
	}

	state.count++
	return rateLimitDecision{
		allowed:   true,
		limit:     l.limit,
		remaining: l.limit - state.count,
		resetAt:   resetAt,
	}
}

func (l *clientRateLimiter) prune(now time.Time) {
	for client, state := range l.clients {
		if now.Sub(state.lastSeen) > 2*l.window {
			delete(l.clients, client)
		}
	}
}

func (l *clientRateLimiter) evictOldestIfFull() {
	if l.maxClients <= 0 || len(l.clients) < l.maxClients {
		return
	}

	var oldestClient string
	var oldestSeen time.Time
	for client, state := range l.clients {
		if oldestClient == "" ||
			state.lastSeen.Before(oldestSeen) ||
			(state.lastSeen.Equal(oldestSeen) && client < oldestClient) {
			oldestClient = client
			oldestSeen = state.lastSeen
		}
	}

	if oldestClient != "" {
		delete(l.clients, oldestClient)
	}
}
