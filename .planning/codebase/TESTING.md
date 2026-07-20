# Testing Patterns

**Analysis Date:** 2026-07-20

## Overview

Only the Go backend has tests. The frontend (Svelte) and community site (Astro) have **zero** tests — no test framework is configured and no test files exist.

| Area | Framework | Tests | Coverage |
|------|-----------|-------|----------|
| `beamsync/` (Go core library) | `testing` (stdlib) | 30+ tests, 1 benchmark | Substantial — all modules tested |
| `desktop/` (Go Wails app) | None detected | 0 tests | Untested |
| `desktop/frontend/` (Svelte) | None | 0 tests | Untested |
| `community/` (Astro) | None | 0 tests | Untested |

---

## Go Test Framework

**Runner:**
- Go standard `testing` package (no external test frameworks)
- Run: `go test ./...` from `beamsync/` or `desktop/`
- CI runs tests for both modules

**CI Commands:**
```bash
# beamsync
cd beamsync && go test ./...

# desktop
cd desktop && go test ./...
```

**Assertions:**
- No assertion library — all assertions use standard `t.Fatalf` / `t.Fatal` with descriptive messages
- Pattern: `t.Fatalf("expected X, got %v", got)`

## Test File Organization

**Location:**
- Co-located with source files in the same directory
- Same package (`package beamsync`) — white-box testing (access to unexported identifiers)

**Naming:**
- `{source_file_name}_test.go`: `history_test.go`, `auth_tokens_test.go`, `server_test.go`, `rate_limiter_test.go`, `stats_test.go`, `permissions_test.go`, `port_manager_test.go`, `tls_test.go`, `resumable_upload_test.go`

**Structure:**
```
beamsync/
├── auth_tokens.go
├── auth_tokens_test.go    # Tests for token store
├── history.go
├── history_test.go        # Tests + 1 benchmark for transfer history
├── server.go
├── server_test.go         # Integration tests for HTTP server
├── rate_limiter_test.go   # Tests for rate limiter
├── stats_test.go          # Tests for stats tracker
├── permissions_test.go    # Tests for transfer settings
├── port_manager_test.go   # Tests for port finding
├── tls_test.go            # Tests for TLS/certificate
├── resumable_upload_test.go # Tests for resumable upload
```

## Test Structure

**Table-driven tests** (used when multiple cases):
```go
func TestResumableEndpointsAreRateLimited(t *testing.T) {
    tests := []struct {
        name       string
        method     string
        path       string
        wantBefore int
    }{
        {name: "resumable upload", method: http.MethodPut, path: "/upload/resumable", wantBefore: http.StatusBadRequest},
        {name: "upload status", method: http.MethodGet, path: "/upload-status/test-upload-id", wantBefore: http.StatusNotFound},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            // ...
        })
    }
}
```

**Subtests** with `t.Run`:
```go
func TestProcessManifestCases(t *testing.T) {
    t.Run("valid", func(t *testing.T) { ... })
    t.Run("empty manifest", func(t *testing.T) { ... })
    t.Run("malformed json", func(t *testing.T) { ... })
}
```

**Certificate host tests** use subtests inside a loop:
```go
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        got := isRelevantCertificateIP(net.ParseIP(tc.ip))
        if got != tc.want { t.Fatalf(...) }
    })
}
```

## Test Helpers

**Common patterns:**

```go
// Helper to create test fixtures — calls t.Helper()
func newTokenStoreForTest(t *testing.T) *tokenStore {
    t.Helper()
    store, err := newTokenStore("test-fingerprint")
    if err != nil {
        t.Fatalf("newTokenStore: %v", err)
    }
    return store
}

// Server test infrastructure
func startServerForTest(t *testing.T) (*HTTPServer, string, string, <-chan string, string) {
    t.Helper()
    // starts real HTTP server, exchanges bootstrap token
}

func startRawServerForTest(t *testing.T) (*HTTPServer, string, string, <-chan string, string) {
    t.Helper()
    // starts real HTTP server, returns bootstrap token
}

func freePort(t *testing.T) int {
    t.Helper()
    // Finds a free TCP port
}

func extractInjectedToken(t *testing.T, html string) string { t.Helper() }
func multipartUploadBody(t *testing.T, filename string, payload []byte) (bytes.Buffer, string) { t.Helper() }
func sha256String(payload []byte) string { ... }
func assertFileMode(t *testing.T, path string, want os.FileMode) { t.Helper() }
```

## Integration-Style Testing

The server tests start **real HTTP servers** (not mocks) and exercise full request/response cycles:

```go
func TestStartServerLifecycleRootAndHeartbeat(t *testing.T) {
    server, baseURL, bootstrapToken, events, _ := startRawServerForTest(t)

    resp, err := http.Get(baseURL + "/?token=" + bootstrapToken)
    defer resp.Body.Close()

    // Assert response headers, body, status code
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("GET / status = %d", resp.StatusCode)
    }
}
```

**Event channel testing** for async verification:
```go
func waitForEvent(events <-chan string, want string, timeout time.Duration) bool {
    timer := time.NewTimer(timeout)
    defer timer.Stop()
    for {
        select {
        case event := <-events:
            if strings.HasPrefix(event, want+"|") {
                return true
            }
        case <-timer.C:
            return false
        }
    }
}
```

## Mocking

**No mocking framework used.** The codebase avoids mocks by:
- Using real implementations with environment variable overrides (`t.Setenv(tlsEnvVar, "")`)
- Using `t.TempDir()` for filesystem isolation
- Injecting `time.Now()` overrides via function fields (`now func() time.Time`)
- Using `httptest.NewRecorder` / `httptest.NewRequest` for HTTP handler tests
- Using `net.Listen("tcp", "127.0.0.1:0")` for real listener tests

**What to Mock (current pattern):**
- Time functions via `limiter.now = func() time.Time { return now }`
- Environment via `t.Setenv`

**What NOT to Mock (current pattern):**
- HTTP servers — tested via real `httptest` or `net.Listen` + `http.Serve`
- File system — tested via `t.TempDir()` and real `os` operations
- Multipart form encoding — built with real `mime/multipart` writers

## Fixtures and Factories

**Test data** is constructed inline — no fixture files:

```go
history.Add(TransferRecord{
    Filename:  "file.txt",
    Direction: TransferDirectionReceive,
    Status:    TransferStatusCompleted,
})
```

**Multipart payloads** built programmatically in helpers:
```go
func multipartUploadBody(t *testing.T, filename string, payload []byte) (bytes.Buffer, string) {
    t.Helper()
    var body bytes.Buffer
    writer := multipart.NewWriter(&body)
    // ... build manifest + file parts
    return body, writer.FormDataContentType()
}
```

**Channel-based event capture** for verifying async events:
```go
events := make(chan string, 20)
emit := func(name, data string) {
    events <- name + "|" + data
}
// later:
if !waitForEvent(events, "device_connected", time.Second) {
    t.Fatal("device_connected event was not emitted")
}
```

## Coverage

**No coverage threshold enforced in CI.** There is no coverage configuration file or coverage badge.

To view coverage locally:
```bash
cd beamsync
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Benchmark Tests

One benchmark exists in `history_test.go`:
```go
func BenchmarkTransferHistoryAddAtCapacity(b *testing.B) {
    history := NewTransferHistory(100)
    // warmup
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        history.Add(TransferRecord{...})
    }
}
```

Run with:
```bash
go test -bench=. ./...
```

## Test Types

**Unit Tests:**
- Cover individual components in isolation: `history_test.go`, `auth_tokens_test.go`, `permissions_test.go`, `stats_test.go`, `port_manager_test.go`, `tls_test.go`, `resumable_upload_test.go`, `rate_limiter_test.go`
- Test specific behaviors: expiration, scope binding, collision avoidance, capacity limits, edge cases

**Integration Tests:**
- Full server lifecycle: `TestStartServerLifecycleRootAndHeartbeat` — starts real server, exchanges tokens, verifies events
- Upload flow: `TestUploadWithFileSavesToDiskAndEmitsEvents` — multipart upload, file on disk, stats verified
- Sender flow: `TestStartSenderIssuesClientBoundSingleUseDownloadToken` — download link, single-use enforcement
- Integrity verification: `TestUploadWithMatchingSHA256HeaderSucceeds`, `TestUploadWithMismatchedSHA256HeaderRejectsAndRecordsFailure`
- Rate limiting end-to-end: `TestResumableEndpointsAreRateLimited` — 12 requests then verify 429

**E2E Tests:**
- Not present. No Playwright / browser-based testing

## Common Patterns

**Async Verification:**
```go
func TestSafeEmitDoesNotSpawnPerEventGoroutines(t *testing.T) {
    block := make(chan struct{})
    started := make(chan struct{}, 1)
    // ...
    before := runtime.NumGoroutine()
    // trigger 50 emits
    after := runtime.NumGoroutine()
    if growth := after - before; growth > 5 {
        t.Fatalf("goroutine count grew by %d", growth)
    }
}
```

**Graceful Shutdown Verification:**
```go
func TestHTTPServerShutdownWaitsForActiveRequest(t *testing.T) {
    // Start real server, send request, verify shutdown blocks until complete
}
```

**TLS Certificate Verification:**
```go
func TestGenerateSelfSignedCertificateIncludesHosts(t *testing.T) {
    // Parse x509 certificate, verify hostnames, key type, validity period
}
```

**Rate Limit Header Verification:**
```go
func TestRateLimitMiddlewareReturns429(t *testing.T) {
    // Verify X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After
}
```

---

*Testing analysis: 2026-07-20*
