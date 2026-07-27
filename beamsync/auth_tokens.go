package beamsync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

var (
	errInvalidToken = fmt.Errorf("invalid token")
	errExpiredToken = fmt.Errorf("expired token")
	errUsedToken    = fmt.Errorf("token already used")
)

const defaultTokenTTL = 5 * time.Minute

type tokenScope string

const (
	tokenScopeBootstrap tokenScope = "bootstrap"
	tokenScopeSession   tokenScope = "session"
	tokenScopeTransfer  tokenScope = "transfer"
)

type tokenRecord struct {
	Value     string
	ExpiresAt time.Time
	MaxUses   int
	UseCount  int
	Scope     tokenScope
}

type tokenStore struct {
	mu     sync.Mutex
	ttl    time.Duration
	now    func() time.Time
	tokens map[string]*tokenRecord
}

func newTokenStore(_ string) (*tokenStore, error) {
	return &tokenStore{
		ttl:    defaultTokenTTL,
		now:    time.Now,
		tokens: make(map[string]*tokenRecord),
	}, nil
}

func (s *tokenStore) issue(scope tokenScope, maxUses int, _ string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	now := s.now()
	record := &tokenRecord{
		Value:     hex.EncodeToString(b),
		ExpiresAt: now.Add(s.ttl),
		MaxUses:   maxUses,
		Scope:     scope,
	}

	s.mu.Lock()
	s.tokens[record.Value] = record
	s.mu.Unlock()
	return record.Value, nil
}

func (s *tokenStore) validate(value, _ string, scope tokenScope, consume bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.tokens[value]
	if !ok || record.Scope != scope {
		return errInvalidToken
	}
	if !s.now().Before(record.ExpiresAt) {
		delete(s.tokens, value)
		return errExpiredToken
	}
	if record.MaxUses > 0 && record.UseCount >= record.MaxUses {
		return errUsedToken
	}
	if consume {
		record.UseCount++
	}
	return nil
}

func (s *tokenStore) cleanupExpired() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for value, record := range s.tokens {
		if !now.Before(record.ExpiresAt) || (record.MaxUses > 0 && record.UseCount >= record.MaxUses) {
			delete(s.tokens, value)
		}
	}
}

func (s *tokenStore) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupExpired()
			}
		}
	}()
}
