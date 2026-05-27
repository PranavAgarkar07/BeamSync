package beamsync

import (
	"sync"
	"time"
)

const defaultSessionTokenTimeout = 10 * time.Minute

type sessionToken struct {
	value        string
	timeout      time.Duration
	mu           sync.Mutex
	lastActivity time.Time
}

func newSessionToken(value string, timeout time.Duration) *sessionToken {
	return &sessionToken{
		value:        value,
		timeout:      timeout,
		lastActivity: time.Now(),
	}
}

func (s *sessionToken) Value() string {
	return s.value
}

func (s *sessionToken) Touch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isExpiredLocked(time.Now()) {
		return false
	}
	s.lastActivity = time.Now()
	return true
}

func (s *sessionToken) Validate(value string) (valid bool, expired bool) {
	if value != s.value {
		return false, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.isExpiredLocked(now) {
		return false, true
	}
	s.lastActivity = now
	return true, false
}

func (s *sessionToken) IsExpired() bool {
	if s.timeout <= 0 {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isExpiredLocked(time.Now())
}

func (s *sessionToken) isExpiredLocked(now time.Time) bool {
	return s.timeout > 0 && now.Sub(s.lastActivity) > s.timeout
}
