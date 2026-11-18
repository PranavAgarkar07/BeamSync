# Phase 4 — Deferred Items

Items surfaced during Phase 4 execution and verification that are **outside the phase charter**
("split `server.go`; no behavior changes") and were therefore not fixed here.

Neither is a Phase 4 regression: both code paths are byte-identical to the pre-phase baseline.

---

## DEF-04-01 — No concurrent test for `clientRateLimiter`

**Where:** `beamsync/rate_limiter.go` (`allow`, `prune`, `evictOldestIfFull`)
**Found by:** 04-VERIFICATION.md, Human Verification item 1
**Severity:** low (coverage gap, not a defect)

`-race` cannot prove the absence of a race in code it never runs in parallel. No test in
`rate_limiter_test.go` or `middleware_test.go` starts a goroutine, so the limiter's mutex-guarded
map has never been exercised concurrently.

The limiter moved out of `server.go` in 04-02 as a byte-identical block; nothing about its locking
changed in this phase.

**Suggested fix:** one test constructing a `clientRateLimiter`, running ~16 goroutines through
`allow()` with a mix of shared and distinct keys, while another goroutine calls `prune()` and
`evictOldestIfFull()`. Assert no race output and a per-window counter consistent with the number of
allowed requests. Note D-13: add a new test name, do not rename an existing one.

---

## DEF-04-02 — Missing-file download 404 is untested

**Where:** `beamsync/download_handler.go:30-33`
**Found by:** 04-VERIFICATION.md, Human Verification item 2
**Severity:** low (coverage gap on an information-disclosure-adjacent path)

```go
file, err := os.Open(filePath)
if err != nil {
    http.Error(w, "File not found", http.StatusNotFound)
    return
}
```

No test asserts this branch. It was inspection-only in 04-08's `must_haves` (`truths[0]`), and the
04-08 normalized diff proves it is unchanged from `server.go:1452-1455` — but no request has ever
been made against it in the suite.

**Suggested fix:** against a live `StartServerE` with a valid client-bound transfer token, `GET` a
path that does not exist and assert `404` plus the `File not found` body, and that the deferred
`transfer_stats` emit still fires. A test at the `httptest` level would need the handler reachable
without a real sender; otherwise extend the existing sender test.

---

## Non-issues considered and dismissed in Phase 4

- **`beamsync/firewall_setup.sh` vs `build/linux/firewall_setup.sh`** — a real duplicate, but the
  `beamsync/` copy is the `//go:embed` source and cannot be deleted. Deferred to Phase 8 `DOC-03`
  by decision, not by oversight.
- **`getLocalIPForURL` has no caller in `server.go`** — its only caller is `startBootstrapRotator`,
  which moved to `events.go` in 04-01. Not an orphan; the plan's description of the call site was
  stale.
- **`tokenMiddleware` (single-scope) has zero production call sites** — verified pre-existing at the
  1538-line baseline (`server.go:519`); only `server_test.go` calls it. Left alone per the
  no-behavior-change charter.
- **`server.go` is 698 lines, not ~200** — `StartServerE` and `StartSenderE` retain 10 inline route
  closures (5 each). Those are "route registration", explicitly in scope for the file to keep per
  ROADMAP 4.9, not extraction candidates.
- **`STATE.md`'s Phase 4 row read `2/10 reqs`** — stale from milestone creation; corrected during
  04-07's tracking update and again at phase close.
- **Local `golangci-lint` v1.64.8 cannot decode Go 1.27.4 export data** — environmental, affects
  untouched files equally. `go build` + `go vet` + `go test -race` + `gofmt -l` were the
  authoritative gate all phase. CI lint needs `GOTOOLCHAIN=go1.25.5`.
