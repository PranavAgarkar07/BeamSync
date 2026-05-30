package beamsync

import (
	"sync"
	"time"
)

const defaultSessionTokenTimeout = 30 * time.Minute

type sessionClock interface {
	Now() time.Time
}

type realSessionClock struct{}

func (realSessionClock) Now() time.Time {
	return time.Now()
}

type sessionToken struct {
	value        string
	timeout      time.Duration
	clock        sessionClock
	mu           sync.Mutex
	lastActivity time.Time
}

func newSessionToken(value string, timeout time.Duration) *sessionToken {
	return newSessionTokenWithClock(value, timeout, realSessionClock{})
}

func newSessionTokenWithClock(value string, timeout time.Duration, clock sessionClock) *sessionToken {
	if clock == nil {
		clock = realSessionClock{}
	}
	return &sessionToken{
		value:        value,
		timeout:      timeout,
		clock:        clock,
		lastActivity: clock.Now(),
	}
}

func (s *sessionToken) Value() string {
	return s.value
}

func (s *sessionToken) Touch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	if s.isExpiredLocked(now) {
		return false
	}
	s.lastActivity = now
	return true
}

func (s *sessionToken) Validate(value string) (valid bool, expired bool) {
	if value != s.value {
		return false, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
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
	return s.isExpiredLocked(s.clock.Now())
}

func (s *sessionToken) isExpiredLocked(now time.Time) bool {
	return s.timeout > 0 && now.Sub(s.lastActivity) > s.timeout
}
