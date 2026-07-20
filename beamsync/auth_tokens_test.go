package beamsync

import (
	"errors"
	"testing"
	"time"
)

func newTokenStoreForTest(t *testing.T) *tokenStore {
	t.Helper()
	store, err := newTokenStore("test-fingerprint")
	if err != nil {
		t.Fatalf("newTokenStore: %v", err)
	}
	return store
}

func TestTokenStoreIssuesAndValidates(t *testing.T) {
	store := newTokenStoreForTest(t)
	token, err := store.issue(tokenScopeSession, 0, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if err := store.validate(token, "", tokenScopeSession, false); err != nil {
		t.Fatalf("validate valid token: %v", err)
	}
}

func TestTokenStoreRejectsInvalidToken(t *testing.T) {
	store := newTokenStoreForTest(t)
	if err := store.validate("badtoken", "", tokenScopeSession, false); !errors.Is(err, errInvalidToken) {
		t.Fatalf("invalid-token error = %v, want %v", err, errInvalidToken)
	}
}

func TestTokenStoreRejectsWrongScope(t *testing.T) {
	store := newTokenStoreForTest(t)
	token, err := store.issue(tokenScopeSession, 0, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if err := store.validate(token, "", tokenScopeTransfer, false); !errors.Is(err, errInvalidToken) {
		t.Fatalf("wrong-scope error = %v, want %v", err, errInvalidToken)
	}
}

func TestTokenStoreExpiresTokens(t *testing.T) {
	store := newTokenStoreForTest(t)
	now := time.Unix(1_700_000_000, 0)
	store.now = func() time.Time { return now }
	store.ttl = time.Minute
	token, err := store.issue(tokenScopeSession, 0, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	now = now.Add(time.Minute)
	if err := store.validate(token, "", tokenScopeSession, false); !errors.Is(err, errExpiredToken) {
		t.Fatalf("expired-token error = %v, want %v", err, errExpiredToken)
	}
}

func TestTokenStoreEnforcesSingleUse(t *testing.T) {
	store := newTokenStoreForTest(t)
	token, err := store.issue(tokenScopeTransfer, 1, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if err := store.validate(token, "", tokenScopeTransfer, true); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := store.validate(token, "", tokenScopeTransfer, true); !errors.Is(err, errUsedToken) {
		t.Fatalf("replay error = %v, want %v", err, errUsedToken)
	}
}

func TestTokenStoreRejectsTampering(t *testing.T) {
	store := newTokenStoreForTest(t)
	token, err := store.issue(tokenScopeSession, 0, "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	tampered := "x" + token[1:]
	if err := store.validate(tampered, "", tokenScopeSession, false); !errors.Is(err, errInvalidToken) {
		t.Fatalf("tampered-token error = %v, want %v", err, errInvalidToken)
	}
}
