---
phase: 03-tests-before-refactoring
reviewed: 2026-11-13
depth: deep
status: clean
files_reviewed: 12
files_reviewed_list:
  - beamsync/event_dispatcher_test.go
  - beamsync/progress_test.go
  - beamsync/progress_integration_test.go
  - beamsync/progress_wiring_test.go
  - beamsync/permissions_test.go
  - beamsync/rate_limiter_test.go
  - beamsync/server.go
  - desktop/app_test.go
  - desktop/app.go
  - desktop/main.go
  - .github/workflows/ci.yml
  - .gitignore
---

# Code Review — Phase 3: Tests Before Refactoring

## Verdict

**`status: clean`** — 0 BLOCKER, 0 WARNING.

The phase goal (a trustworthy regression safety net before Phase 4's file-splitting) is
met. Every production behaviour this phase introduced or repaired is pinned by a test
that fails when the behaviour is broken — verified by mutation, not by reading the diff.

## Mutation campaign

**33 requested mutants, 33 killed.** Each mutant was applied to a throwaway copy and the
suite re-run; a kill means the suite went red.

| # | Mutant | Killed by |
|---|--------|-----------|
| 1 | receive-path `eventName` value | `TestReceivePathEmitsUploadProgress` |
| 2 | single-file send `eventName` value | `TestSenderPathEmitsDownloadProgress` |
| 3 | multi-file send `eventName` value | `TestMultiFileSenderPathEmitsDownloadProgress` |
| 4 | `d.dropped.Add(1)` removal | 3 dispatcher tests |
| 5 | `progressWriter` zero-byte guard removal | `TestProgressWriterZeroByteWriteEmitsNothing` |
| 6 | `pw.written += int64(len(p))` | `TestProgressWriterCountsBytesActuallyWritten` |
| 7 | `eventDispatcher.close()` a no-op | `TestEventDispatcherCloseStopsWorkerAndIsIdempotent` |
| 8 | `eventDispatcher` RWMutex removed | `TestEventDispatcherCloseRacesEmitSafely` (real `DATA RACE`) |
| 9 | `parseVersion` returns the fallback | `TestParseVersionUsesProductVersionNotFallback` |
| 10 | `initVersion` stores the fallback | `TestInitVersionStoresParsedVersion` |
| 11 | `endpointMu` lock removed | `TestProcessEventsTracksTokenAndPortFromURLChange` (real `DATA RACE`) |
| 12 | `serverMu` lock removed | `TestServerAndSavePathAccessorsAreRaceFree` (real `DATA RACE`) |
| 13 | `stateMu` lock removed | `TestVersionAndAudioAccessorsAreRaceFree` (real `DATA RACE`) |
| 14 | `settingsMu` lock removed | `TestUpdateSettingsIsRaceFreeWithConcurrentReaders` (real `DATA RACE`) |
| 15 | `Settings()` returns the live pointer | `TestSettingsReturnsSnapshotNotLivePointer` + 14 (real `DATA RACE`) |
| 16 | `extractPortFromURL` IPv6 branch removed | `TestExtractPortFromURL` |
| 17 | home-guard boundary → raw prefix | `TestLoadConfigClearsSavePathInHomeSibling` |
| 18 | receive-path terminal emit removed | `TestReceivePathEmitsCompletionProgress` |
| 19 | multi-file send terminal emit removed | `TestMultiFileSenderPathEmitsCompletionProgress` |
| 20 | receive-path declared `total` forced to 0 | `TestReceivePathReportsDeclaredTotal` |
| 21 | ordered manifest matching → lossy keying | `TestReceivePathGivesEachFileItsOwnDeclaredSize` |
| 22 | `processEvents` device_connected branch removed | `TestProcessEventsDetectsIPChange` |
| 23 | `processEvents` `|| token_rotated` removed | `TestProcessEventsTracksTokenAndPortFromURLChange` |
| 24 | `handleIPChange` port/token order swapped | `TestProcessEventsPublishesRefreshedURLOnIPChange` |
| 25 | `handleIPChange` port guard removed | `TestHandleIPChangeDoesNotPublishWithoutPort` |
| 26 | `startIPMonitor` reverted to duplicated logic | `TestStartIPMonitorUsesSharedIPChangeHandling` |
| 27 | `stopReceiver` re-locks `serverMu` | `TestStopReceiverAndSenderDoNotDeadlock` (hangs → 5 s bound) |
| 28 | `updateCurrentEndpoint` drops the token | `TestUpdateCurrentEndpointSetsPortAndToken` |
| 29 | `processEvents` returns after the first event | `TestProcessEventsRelaysEveryEvent` |
| 30 | `shutdown()` deadlocks | `TestShutdownStopsServersAndClosesChannels` (hangs → 5 s bound) |
| 31 | `shutdownOnce` guard removed | `TestShutdownIsIdempotent` (`close of closed channel`) |
| 32 | `makeCallback` quit guard → single select | `TestMakeCallbackDropsEventsAfterQuit` |
| 33 | `rateLimitMiddleware` captures a pointer | `TestRateLimitMiddlewareReadsCurrentSettings` |

## Live defects found and fixed during this phase

Every one of these was a real defect in the shipping app, not a test-only issue.

| Defect | Impact | Fix |
|--------|--------|-----|
| `download_progress` emitted by nothing | Desktop send-direction progress never moved (Phase 1 deleted `downloadProgressWriter`) | `progressWriter.eventName` set per path at all three construction sites |
| Fast sends emitted **zero** progress events | The send panel stayed dead for transfers under 500 ms | Send path no longer re-seeds `lastEmit`; both send paths emit a completion event |
| Upload manifest sizes parsed then discarded | `progressWriter.total` was 0 → upload progress bar indeterminate for the whole transfer | The declared size is now used, matched to parts in order |
| Two uploads sharing a base name | The second file reported the first file's size | Manifest matched in order, entries consumed |
| `serverMu` deadlock | Every Stop/restart/choose-folder/quit path hung with a receiver running | Fields cleared directly under the lock |
| `TransferSettings` written through a live pointer | Torn reads could mis-apply the device blocklist | `Settings()` returns a snapshot; `UpdateSettings` swaps atomically |
| `currentIP`/`currentPort`/`currentToken` racy | QR URL could show a half-updated endpoint | `endpointMu` + snapshot/set accessors |
| `lastSavePath`/`serverApp`/`senderApp` racy | Wails dispatches every call on its own goroutine | `serverMu` + accessors |
| `ctx`/`audio`/`version` racy | `GetVersion`/`PlaySound` raced `OnStartup` | `stateMu` + accessors |
| Quit guard non-deterministic | 92 of 200 events were still enqueued after quit (random `select`) | The quit signal is checked in its own `select` first |
| `extractPortFromURL` returned `1]` | Bracketed IPv6 host with no port mis-parsed | Bracketed literals parsed explicitly |
| Home guard used a raw string prefix | `<home>-evil` counted as inside home | Compared on a path separator boundary |
| CI red on `main` and `dev` | `errorlint` violations + a `typecheck` failure on the `/v2` import | Both fixed; lint clean in both modules |

## Locking and accessor audit

- `desktop/app.go` — 52 `func (a *App) X(...)` methods: **0 self-calls**, every
  lock-taking method is a leaf accessor, **no `*App` method is invoked while holding a lock**.
  (A locking accessor calling another locking accessor caused two bugs in this phase:
  a self-deadlock in `stopReceiver` and one in `setWailsContext`.)
- `beamsync/server.go` — same audit: `Settings`, `UpdateSettings` and `currentSettings` are
  leaf accessors; `rateLimitMiddleware` takes a provider so no pointer is captured at
  route-registration time.

## Known INFO (accepted, not fixed)

- **IN-01 — the Wails publish cannot be unit tested.** `runtime.EventsEmit` →
  `getEvents` → `log.Fatalf` → `os.Exit(1)`: the runtime terminates the process on an
  invalid context, and `recover()` cannot intercept it. The publish path is therefore
  observed through the `urlChangedHook` seam rather than the runtime itself.
- **IN-02 — the `|`-in-filename payload ambiguity.** The progress payload is
  `|`-delimited; a filename containing `|` is ambiguous. Recorded as a deviation in
  `.planning/WINDOWS.md`.
- **IN-03 — CI is Linux-only.** The Windows-only branch of `desktop/app.go` is not
  exercised by CI. Phase 7 scope.
- **IN-04 — `startup()` re-fetches the audio engine** through the accessor on each use.
  Unreachable today (the engine is set first), but a latent startup panic.

## Verification

```
cd beamsync && go test -race ./...          # ok
cd desktop   && go test -race .             # ok
cd beamsync && golangci-lint run ./...      # exit 0
cd desktop   && golangci-lint run .         # exit 0
gofmt -l beamsync desktop                   # clean
go test -race -shuffle=on -count=6 (both)   # 0 failures
GOOS=windows go build ./... (both)          # ok
```

**115 test functions** across both modules.

---
*Phase: 03-Tests Before Refactoring*
*Reviewed: 2026-11-13*
