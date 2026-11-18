package beamsync

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func extractToken(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	if t := r.Header.Get("X-BeamSync-Token"); t != "" {
		return t
	}
	if t := r.Header.Get("Authorization"); strings.HasPrefix(t, "Bearer ") {
		return t[7:]
	}
	return ""
}

func tokenMiddleware(tokens *tokenStore, scope tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		token := extractToken(r)
		if err := tokens.validate(token, "", scope, consume); err != nil {
			fmt.Printf("🔑 TOKEN REJECTED: path=%s token=%q reason=%v\n", r.URL.Path, token, err)
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "403 Forbidden: invalid token", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func tokenMiddlewareAny(tokens *tokenStore, scopes []tokenScope, consume bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		token := extractToken(r)
		if err := tokens.validateAny(token, scopes, consume); err != nil {
			fmt.Printf("🔑 TOKEN REJECTED: path=%s token=%q reason=%v auth=%q x-bst=%q\n", r.URL.Path, token, err, r.Header.Get("Authorization"), r.Header.Get("X-BeamSync-Token"))
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "403 Forbidden: invalid token", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func setRateLimitHeaders(w http.ResponseWriter, decision rateLimitDecision) {
	if decision.limit <= 0 {
		return
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.resetAt.Unix(), 10))
}

// rateLimitMiddleware wraps a handler with per-client rate limiting.
//
// settings is a PROVIDER, not a pointer: the settings can be replaced while
// the server runs, so each request must read the current snapshot rather than
// a pointer captured at route-registration time.
func rateLimitMiddleware(lim *clientRateLimiter, settings func() *TransferSettings, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ip := clientIP(r)
		var current *TransferSettings
		if settings != nil {
			current = settings()
		}
		if current != nil && current.isDeviceTrusted(ip) {
			next(w, r)
			return
		}

		decision := lim.allow(ip)
		setRateLimitHeaders(w, decision)
		if !decision.allowed {
			retryAfter := decision.retryAfter
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			seconds := int64(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      "rate_limit_exceeded",
				"message":    "429 Too Many Requests: rate limit exceeded",
				"retryAfter": seconds,
			})
			return
		}

		next(w, r)
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func clientIP(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	for _, part := range strings.Split(forwardedFor, ",") {
		ip := strings.TrimSpace(part)
		if ip == "" {
			continue
		}
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.String()
		}
		if host, _, err := net.SplitHostPort(ip); err == nil {
			if parsed := net.ParseIP(host); parsed != nil {
				return parsed.String()
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
