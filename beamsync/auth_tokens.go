package beamsync

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultTokenTTL = 5 * time.Minute

type tokenScope string

const (
	tokenScopeBootstrap tokenScope = "bootstrap"
	tokenScopeSession   tokenScope = "session"
	tokenScopeTransfer  tokenScope = "transfer"
)

var (
	errInvalidToken = errors.New("invalid token")
	errExpiredToken = errors.New("expired token")
	errUsedToken    = errors.New("token use limit exceeded")
	errWrongClient  = errors.New("token is bound to another client")
)

type tokenRecord struct {
	Value            string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	MaxUses          int
	UseCount         int
	BoundFingerprint string
	BoundClientIP    string
	Scope            tokenScope
	Nonce            string
}

type tokenStore struct {
	mu          sync.Mutex
	secret      []byte
	sessionID   string
	fingerprint string
	ttl         time.Duration
	now         func() time.Time
	tokens      map[string]*tokenRecord
}

func newTokenStore(fingerprint string) (*tokenStore, error) {
	secret, err := randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("generate token secret: %w", err)
	}
	sessionIDBytes, err := randomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("generate token session ID: %w", err)
	}
	return &tokenStore{
		secret:      secret,
		sessionID:   hex.EncodeToString(sessionIDBytes),
		fingerprint: fingerprint,
		ttl:         defaultTokenTTL,
		now:         time.Now,
		tokens:      make(map[string]*tokenRecord),
	}, nil
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *tokenStore) issue(scope tokenScope, maxUses int, clientIP string) (string, error) {
	nonceBytes, err := randomBytes(16)
	if err != nil {
		return "", fmt.Errorf("generate token nonce: %w", err)
	}

	now := s.now()
	record := &tokenRecord{
		CreatedAt:        now,
		ExpiresAt:        now.Add(s.ttl),
		MaxUses:          maxUses,
		BoundFingerprint: s.fingerprint,
		BoundClientIP:    clientIP,
		Scope:            scope,
		Nonce:            hex.EncodeToString(nonceBytes),
	}
	record.Value = s.sign(record)

	s.mu.Lock()
	s.tokens[record.Value] = record
	s.mu.Unlock()
	return record.Value, nil
}

func (s *tokenStore) sign(record *tokenRecord) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		s.sessionID,
		record.BoundFingerprint,
		record.BoundClientIP,
		record.Scope,
		record.ExpiresAt.Unix(),
		record.Nonce,
	)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil)) + "." + record.Nonce
}

func (s *tokenStore) validate(value, clientIP string, scope tokenScope, consume bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.tokens[value]
	if !ok || record.Scope != scope || !hmac.Equal([]byte(value), []byte(s.sign(record))) {
		return errInvalidToken
	}
	if !s.now().Before(record.ExpiresAt) {
		delete(s.tokens, value)
		return errExpiredToken
	}
	if record.BoundFingerprint != s.fingerprint {
		return errInvalidToken
	}
	if record.BoundClientIP != "" && record.BoundClientIP != clientIP {
		return errWrongClient
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
