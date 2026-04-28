---
phase: 03-centrifugo-integration
reviewed: 2026-04-27T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/protocol/envelope.go
  - internal/protocol/envelope_test.go
  - centrifugo/config.json
  - cmd/bob-classical/main.go
  - cmd/alice-classical/main.go
  - cmd/bob-pq/main.go
  - cmd/alice-pq/main.go
findings:
  critical: 2
  warning: 2
  info: 1
  total: 5
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-04-27T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Seven files were reviewed: the protocol package (envelope + tests), the Centrifugo config, and all four cmd entrypoints. The protocol package itself is clean and well-tested. The Centrifugo config has no secrets but runs in insecure mode. The four cmd entrypoints share two structural bugs: a data race on the `sess` pointer across goroutines, and `os.Exit(0)` bypassing `defer cl.Disconnect()`. Channel isolation (classical vs PQ channels) is correct — no cross-channel publishing was found. `sync/atomic` usage for `echoCount`/`recvCount` is correct. The nil-guard for `sess` before `Decrypt` exists in all four cmd files but is itself subject to the data race described below.

---

## Critical Issues

### CR-01: Data race on `sess` pointer in bob-classical and bob-pq

**Files:**
- `cmd/bob-classical/main.go:69` (write), `cmd/bob-classical/main.go:73-82` (read)
- `cmd/bob-pq/main.go:69` (write), `cmd/bob-pq/main.go:74-82` (read)

**Issue:** The subscription callback is `go func()` — every incoming message spawns an independent goroutine. Two message goroutines can execute concurrently: one executing the `TypeInitialMsg` arm writes `sess = s` (line 69 in both files), while another executing the `TypeRatchetMsg` arm reads `sess` for the nil-guard and for `sess.Decrypt`. Both arms run in unsynchronized goroutines sharing the same `sess` variable. This is a data race under the Go memory model: no mutex, no `sync/atomic`, no channel sequencing protects the shared pointer. The race detector (`go test -race` or running with `GORACE`) will flag this. Beyond the race detector, the practical risk is that a ratchet_msg arriving before or during handshake could read a partially-written session pointer.

**Fix:** Protect `sess` with a `sync.Mutex`:

```go
var (
    sessMu sync.Mutex
    sess   *classical.Session  // or *pq.Session in bob-pq
)

// In TypeInitialMsg arm:
sessMu.Lock()
sess = s
sessMu.Unlock()

// In TypeRatchetMsg arm:
sessMu.Lock()
currentSess := sess
sessMu.Unlock()
if currentSess == nil {
    log.Printf("bob-classical: session not yet initialized, dropping ratchet_msg")
    return
}
plain, err := currentSess.Decrypt(&msg)
```

Alternatively, since the Session type itself may not be goroutine-safe (double-ratchet state mutates on every Decrypt), the mutex must also guard the `Decrypt` and `Encrypt` calls — see WR-01 below.

---

### CR-02: Data race on `sess` pointer in alice-classical and alice-pq

**Files:**
- `cmd/alice-classical/main.go:108` (write in main goroutine), `cmd/alice-classical/main.go:65-74` (read in callback goroutine)
- `cmd/alice-pq/main.go:113` (write in main goroutine), `cmd/alice-pq/main.go:66-75` (read in callback goroutine)

**Issue:** Alice writes `sess = s` in the main goroutine (after receiving from `bundleCh`). Concurrently, subscription callback goroutines may already be spawned and reading `sess` for the nil-guard and `Decrypt`. The channel receive from `bundleCh` provides a happens-before for the goroutine that wrote to `bundleCh`, but it does not provide a happens-before between the main goroutine's write to `sess` and any subsequently spawned callback goroutines. In practice the race window is small (echoes only arrive after Alice publishes the initial_msg), but it is still a formal Go data race. Running `go test -race` against an integration harness or using `-race` on the binary will surface it.

**Fix:** Same pattern as CR-01 — guard `sess` with a `sync.Mutex`. In Alice's case the write is in the main goroutine and reads are in callback goroutines:

```go
var sessMu sync.Mutex

// Main goroutine (after InitiatorHandshake):
sessMu.Lock()
sess = s
sessMu.Unlock()

// Callback goroutine TypeRatchetMsg arm:
sessMu.Lock()
currentSess := sess
sessMu.Unlock()
if currentSess == nil { ... }
plain, err := currentSess.Decrypt(&msg)
```

---

## Warnings

### WR-01: `os.Exit(0)` inside goroutines skips all `defer` statements in main

**Files:**
- `cmd/bob-classical/main.go:118`
- `cmd/alice-classical/main.go:88`
- `cmd/bob-pq/main.go:119`
- `cmd/alice-pq/main.go:89`

**Issue:** Each cmd registers `defer cl.Disconnect()` in `main()` (lines 37, 39, 37, 39 respectively). When the exit condition is met (5 echoes sent/received), `os.Exit(0)` is called from inside a goroutine. `os.Exit` terminates the process immediately without running any deferred functions — `cl.Disconnect()` is never called. For a demo this is harmless, but it means the WebSocket connection is torn down abruptly rather than via a clean close handshake, which can leave Centrifugo-side state dirty and makes the pattern non-portable to any production use.

**Fix:** Signal the main goroutine to exit cleanly using a channel:

```go
done := make(chan struct{})

// In the goroutine callback, replace os.Exit(0) with:
if n == 5 {
    log.Printf("bob-classical: sent 5 echoes, signaling done")
    close(done)
    return
}

// Replace `select {}` at the bottom of main with:
<-done
// defer cl.Disconnect() now runs here
```

---

### WR-02: `sess` (double-ratchet state) mutated without a lock — concurrent Decrypt/Encrypt calls possible

**Files:**
- `cmd/bob-classical/main.go:82,101`
- `cmd/bob-pq/main.go:82,103`

**Issue:** Each incoming `ratchet_msg` spawns a new goroutine that calls `sess.Decrypt` followed by `sess.Encrypt`. If two `ratchet_msg` messages arrive close together (e.g., network burst), two goroutines will concurrently call `Decrypt` on the same Session object. Double-ratchet session state is stateful and mutates on every call — the go-doubleratchet library does not document goroutine safety, making concurrent mutation a correctness bug that would corrupt ratchet chain state, not merely a race on the pointer.

Note: Alice-side variants (alice-classical/alice-pq) only call `Decrypt` in the echo-receive arm, not `Encrypt`, and Alice sends messages sequentially from main — lower risk but the same concern applies if two echoes arrived simultaneously.

**Fix:** The mutex introduced for CR-01/CR-02 must also wrap the `Decrypt` and `Encrypt` calls, not just the nil-guard:

```go
sessMu.Lock()
plain, err := sess.Decrypt(&msg)
if err != nil {
    sessMu.Unlock()
    log.Printf("bob-classical: Decrypt: %v", err)
    return
}
echoMsg, err := sess.Encrypt(echoJSON)
sessMu.Unlock()
```

This serializes session operations across goroutines, preserving ratchet ordering.

---

## Info

### IN-01: Centrifugo config uses `"insecure": true` — no authentication

**File:** `centrifugo/config.json:3`

**Issue:** `"client": { "insecure": true }` disables all token-based authentication. Any process that can reach port 8000 can subscribe to or publish on any channel, including both `ch-classical` and `ch-pq`. For a local demo this is intentional, but the setting must not be carried into any shared or production environment without enabling token authentication (JWT or API key).

**Fix:** For production use, remove `"insecure": true` and configure a `token_hmac_secret_key` (or RSA equivalent) plus per-channel subscribe/publish permissions:

```json
{
  "token_hmac_secret_key": "<secret>",
  "client": {},
  "channel": {
    "namespaces": [
      { "name": "ch-classical", "subscribe_for_client": true },
      { "name": "ch-pq", "subscribe_for_client": true }
    ]
  }
}
```

---

_Reviewed: 2026-04-27T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
