package beamsync

import (
	"testing"
	"time"
)

func TestSessionTokenValidateRefreshesActivity(t *testing.T) {
	session := newSessionToken("secret", 40*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	valid, expired := session.Validate("secret")
	if !valid || expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid true expired false", valid, expired)
	}

	time.Sleep(25 * time.Millisecond)
	if session.IsExpired() {
		t.Fatal("validated session should refresh the inactivity window")
	}
}

func TestSessionTokenExpiresAfterInactivity(t *testing.T) {
	session := newSessionToken("secret", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
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
	session := newSessionToken("secret", 0)

	session.lastActivity = time.Now().Add(-time.Hour)
	if session.IsExpired() {
		t.Fatal("zero timeout should disable inactivity expiry")
	}
}

func TestSessionTokenTouchDoesNotReviveExpiredSession(t *testing.T) {
	session := newSessionToken("secret", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	if session.Touch() {
		t.Fatal("Touch() should not revive an expired session")
	}

	valid, expired := session.Validate("secret")
	if valid || !expired {
		t.Fatalf("Validate() = valid %v expired %v, want valid false expired true", valid, expired)
	}
}
