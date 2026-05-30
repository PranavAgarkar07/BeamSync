package beamsync

import (
	"sync"
	"testing"
	"time"
)

type fakeSessionClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeSessionClock() *fakeSessionClock {
	return &fakeSessionClock{now: time.Unix(0, 0)}
}

func (c *fakeSessionClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeSessionClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestSessionTokenValidateRefreshesActivity(t *testing.T) {
	clock := newFakeSessionClock()
	session := newSessionTokenWithClock("secret", time.Minute, clock)

	clock.Advance(30 * time.Second)
	valid, expired := session.Validate("secret")
	if !valid || expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid true expired false", valid, expired)
	}

	clock.Advance(45 * time.Second)
	if session.IsExpired() {
		t.Fatal("validated session should refresh the inactivity window")
	}
}

func TestSessionTokenExpiresAfterInactivity(t *testing.T) {
	clock := newFakeSessionClock()
	session := newSessionTokenWithClock("secret", time.Minute, clock)

	clock.Advance(time.Minute + time.Second)
	valid, expired := session.Validate("secret")
	if valid || !expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid false expired true", valid, expired)
	}
}

func TestSessionTokenRejectsWrongTokenWithoutExpiring(t *testing.T) {
	session := newSessionToken("secret", time.Minute)

	valid, expired := session.Validate("wrong")
	if valid || expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid false expired false", valid, expired)
	}
}

func TestSessionTokenCanDisableTimeout(t *testing.T) {
	clock := newFakeSessionClock()
	session := newSessionTokenWithClock("secret", 0, clock)

	clock.Advance(time.Hour)
	if session.IsExpired() {
		t.Fatal("zero timeout should disable inactivity expiry")
	}
}

func TestSessionTokenTouchDoesNotReviveExpiredSession(t *testing.T) {
	clock := newFakeSessionClock()
	session := newSessionTokenWithClock("secret", time.Minute, clock)

	clock.Advance(time.Minute + time.Second)
	if session.Touch() {
		t.Fatal("Touch() should not revive an expired session")
	}

	valid, expired := session.Validate("secret")
	if valid || !expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid false expired true", valid, expired)
	}
}
