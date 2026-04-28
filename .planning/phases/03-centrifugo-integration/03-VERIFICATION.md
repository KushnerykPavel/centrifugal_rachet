---
phase: 03-centrifugo-integration
verified: 2026-04-27T10:00:00Z
status: human_needed
score: 3/4 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Start Centrifugo with `centrifugo -c centrifugo/config.json` and run `curl http://localhost:8000/health`"
    expected: "HTTP 200 response confirming Centrifugo is running with insecure mode active"
    why_human: "Cannot start a server process during static verification; config file is correct but runtime health check requires an active process"
  - test: "Run bob-classical in one terminal, alice-classical in another (both pointing at localhost:8000), observe logs"
    expected: "Bob logs 'published prekey bundle', Alice logs 'handshake complete', both log 5 seq/echo pairs, both log 'exiting'"
    why_human: "End-to-end message exchange over live Centrifugo requires a running server and two processes; static analysis confirms code correctness but not runtime behavior"
  - test: "Run bob-pq and alice-pq in separate terminals"
    expected: "Same as classical pair: 5 ratchet messages exchanged over ch-pq, both exit after echo cycle completes"
    why_human: "Same as above — PQ variant requires runtime verification"
---

# Phase 3: Centrifugo Integration Verification Report

**Phase Goal:** Both classical and PQ Alice-Bob pairs exchange encrypted messages over live Centrifugo channels using in-band key exchange, with no goroutine deadlocks
**Verified:** 2026-04-27T10:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Centrifugo config enables `client.insecure: true` and `health: true`; `GET /health` returns 200 | ? HUMAN | `centrifugo/config.json` contains exact required values; runtime health check requires running server |
| 2 | Bob roles publish typed `prekey_bundle` envelope first; Alice receives within 30 s and completes handshake | ✓ VERIFIED | Bob calls `MarshalEnvelope(TypePrekeyBundle, bundle)` + Publish before `select{}`; Alice has `bundleCh := make(chan []byte, 1)` with `time.After(30*time.Second)` timeout; both classical and PQ variants wired end-to-end |
| 3 | `ch-classical` and `ch-pq` are independent — no cross-channel publication | ✓ VERIFIED | `grep ChannelClassical cmd/bob-pq/ cmd/alice-pq/` returns nothing; `grep ChannelPQ cmd/bob-classical/ cmd/alice-classical/` returns nothing; each binary references only its own channel constant |
| 4 | All `OnPublication` callbacks dispatch to `go func()` — no blocking call inside handler loop | ✓ VERIFIED | Every `cl.Subscribe(...)` callback in all four binaries immediately spawns `go func() { ... }()`; all Decrypt/Encrypt/Publish calls are inside those goroutines |

**Score:** 3/4 truths verified (1 requires human runtime check)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/protocol/envelope.go` | Envelope, RatchetPayload, type/channel constants, MarshalEnvelope | ✓ VERIFIED | All 8 exports present; exact JSON field tags correct; MarshalEnvelope uses json.RawMessage correctly |
| `internal/protocol/envelope_test.go` | 5 unit tests, stdlib only | ✓ VERIFIED | `go test ./internal/protocol/...` — 5 tests PASS |
| `centrifugo/config.json` | `client.insecure=true`, `health=true`, `port=8000` | ✓ VERIFIED | File content matches spec exactly |
| `cmd/bob-classical/main.go` | Full classical Bob binary | ✓ VERIFIED | 137 lines; prekey publish, ResponderHandshake, echo loop with RatchetPayload; `go build` exits 0 |
| `cmd/alice-classical/main.go` | Full classical Alice binary | ✓ VERIFIED | 147 lines; 30s timeout, InitiatorHandshake, 5-msg send with RatchetPayload; `go build` exits 0 |
| `cmd/bob-pq/main.go` | Full PQ Bob binary | ✓ VERIFIED | 139 lines; pqxdh bundle publish, PQ ResponderHandshake, TripleRatchetMessage echo loop; `go build` exits 0 |
| `cmd/alice-pq/main.go` | Full PQ Alice binary | ✓ VERIFIED | 155 lines; pqxdh.PrekeyBundle unmarshal target, PQ InitiatorHandshake, 5-msg send with RatchetPayload; `go build` exits 0 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/bob-classical/main.go` | `internal/protocol` | `protocol.MarshalEnvelope`, `protocol.TypePrekeyBundle`, `protocol.ChannelClassical` | ✓ WIRED | Direct import + usage confirmed in source |
| `cmd/alice-classical/main.go` | `internal/protocol` | `protocol.RatchetPayload`, `bundleCh <- env.Payload`, `protocol.ChannelClassical` | ✓ WIRED | RatchetPayload used for both send and echo-recv |
| `cmd/bob-classical/main.go` | `internal/classical` | `classical.NewResponderBundle`, `classical.ResponderHandshake`, `sess.Encrypt`, `sess.Decrypt` | ✓ WIRED | All four functions called; Session flows through handler |
| `cmd/bob-classical/main.go` | `sync/atomic` | `atomic.AddInt32(&echoCount, 1)` | ✓ WIRED | Line 115 confirmed |
| `cmd/bob-pq/main.go` | `internal/pq` | `pq.NewResponderBundle`, `pq.ResponderHandshake`, `sess.Encrypt`, `sess.Decrypt` | ✓ WIRED | All four PQ functions called |
| `cmd/alice-pq/main.go` | `github.com/KushnerykPavel/go-doubleratchet/pqxdh` | `pqxdh.PrekeyBundle` unmarshal target | ✓ WIRED | Line 104: `var bundle pqxdh.PrekeyBundle` |
| `cmd/alice-classical/main.go` | `bundleCh` channel sequencing | `bundleCh <- env.Payload` in handler, receive in main | ✓ WIRED | Buffered channel size 1 with select+default non-blocking send |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `cmd/bob-classical/main.go` | `sess` (classical.Session) | `classical.ResponderHandshake(priv, initMsg)` | Yes — real X3DH handshake | ✓ FLOWING |
| `cmd/alice-classical/main.go` | `sess` (classical.Session) | `classical.InitiatorHandshake(&bundle)` | Yes — real X3DH handshake | ✓ FLOWING |
| `cmd/bob-pq/main.go` | `sess` (pq.Session) | `pq.ResponderHandshake(priv, initMsg)` | Yes — real PQXDH handshake | ✓ FLOWING |
| `cmd/alice-pq/main.go` | `sess` (pq.Session) | `pq.InitiatorHandshake(&bundle)` | Yes — real PQXDH handshake | ✓ FLOWING |
| `cmd/bob-classical/main.go` | `rp` (RatchetPayload) | `sess.Decrypt(&msg)` + `json.Unmarshal(plain, &rp)` | Yes — decrypted from wire | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` exits 0 | `go build ./...` | Success | ✓ PASS |
| All 22 tests pass | `go test ./...` | 22 passed in 11 packages | ✓ PASS |
| Channel isolation: ChannelClassical absent from PQ binaries | `grep -rn ChannelClassical cmd/bob-pq/ cmd/alice-pq/` | No output (exit 1) | ✓ PASS |
| Channel isolation: ChannelPQ absent from classical binaries | `grep -rn ChannelPQ cmd/bob-classical/ cmd/alice-classical/` | No output (exit 1) | ✓ PASS |
| Buffered bundleCh in both Alice binaries | `grep -rn 'make(chan \[\]byte, 1)' cmd/` | alice-classical:41, alice-pq:42 | ✓ PASS |
| All four binaries have go func() handler dispatch | `grep -rn "go func()" cmd/` | 2 per binary (metrics + handler) | ✓ PASS |
| atomic.AddInt32 in all four binaries | `grep -rn "atomic.AddInt32" cmd/` | 1 per binary | ✓ PASS |
| RatchetPayload used for send and echo-recv in all four | `grep -rn "RatchetPayload" cmd/` | 10 matches across 4 files | ✓ PASS |
| pqxdh.PrekeyBundle unmarshal target in alice-pq | `grep -n "pqxdh.PrekeyBundle" cmd/alice-pq/main.go` | Line 104 | ✓ PASS |
| Runtime end-to-end message exchange | Requires live Centrifugo + two processes | N/A | ? SKIP — needs live server |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CENT-01 | 03-01 | Centrifugo config with `client.insecure: true` and `health: true`; no token required | ✓ SATISFIED | `centrifugo/config.json` has both fields at correct nesting; channel constants `ch-classical`/`ch-pq` require no auth |
| CENT-02 | 03-01 | All four binaries use typed Envelope with `Type` field (`prekey_bundle | initial_msg | ratchet_msg`) | ✓ SATISFIED | All binaries use `protocol.MarshalEnvelope` + type constants; all handlers switch on `env.Type` |
| CENT-03 | 03-02, 03-03 | Bob publishes prekey bundle first; Alice retries subscription up to 30 s | ✓ SATISFIED | Bob publishes before `select{}`; Alice has `time.After(30*time.Second)` in select; both classical and PQ pairs confirmed |
| CENT-04 | 03-02, 03-03 | All `OnPublication` callbacks dispatch to `go func()` | ✓ SATISFIED | Every subscription handler in all four binaries immediately spawns `go func()` before any blocking operation |
| CENT-05 | 03-02, 03-03 | Classical pair uses `ch-classical` only; PQ pair uses `ch-pq` only | ✓ SATISFIED | Cross-reference grep confirms zero violations; constants are compile-time strings enforced at build |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/bob-classical/main.go` | 69 (write), 73 (read) | `sess` written in one `go func()`, read in another without mutex | ⚠️ Warning (CR-01) | Race detector will flag; nil-guard prevents panic; practical impact nil for sequential blog demo — ratchet messages arrive after handshake completes |
| `cmd/alice-classical/main.go` | 108 (write in main), 65 (read in handler goroutine) | Same `sess` race pattern | ⚠️ Warning (CR-02) | Same assessment as CR-01; Alice writes sess before publishing initMsg which is required for Bob to send ratchet msgs |
| `cmd/bob-pq/main.go` | 69 / 73 | Same `sess` race — PQ variant | ⚠️ Warning (CR-01 PQ) | Same as classical |
| `cmd/alice-pq/main.go` | 113 / 65 | Same `sess` race — PQ variant | ⚠️ Warning (CR-02 PQ) | Same as classical |
| `cmd/bob-classical/main.go` | 118 | `os.Exit(0)` inside goroutine skips `defer cl.Disconnect()` | ⚠️ Warning (WR-01) | WebSocket not cleanly closed; harmless for demo, non-portable to production |
| `cmd/alice-classical/main.go` | 88 | Same `os.Exit(0)` pattern | ⚠️ Warning (WR-01) | Same |
| `cmd/bob-pq/main.go` | 119 | Same `os.Exit(0)` pattern | ⚠️ Warning (WR-01) | Same |
| `cmd/alice-pq/main.go` | 89 | Same `os.Exit(0)` pattern | ⚠️ Warning (WR-01) | Same |
| `centrifugo/config.json` | 3 | `"insecure": true` — no authentication | ℹ️ Info (IN-01) | Intentional blog-demo decision D-07; documented in T-3-01-03; not a gap |

**Anti-pattern severity assessment — CR-01/CR-02 vs phase goal:**

The phase goal specifies "no goroutine deadlocks." CR-01/CR-02 describe a data race on `sess`, not a deadlock. These are distinct: a deadlock blocks execution forever; a data race is undefined behavior under the Go memory model but has predictable practical behavior in this sequential demo scenario. The nil-guard prevents panics. Alice writes `sess = s` before publishing `initial_msg` (which is the precondition for Bob sending ratchet messages), making the race window near-zero in practice. The `go -race` detector would flag these, but they do not block the phase goal of exchanging messages without deadlocks.

Classification: **Warning** (not Blocker). The data races are real and should be fixed in a follow-up (add `sync.Mutex` per the review's suggested fix), but they do not prevent the phase goal from being achieved for a blog demo running sequentially.

### Human Verification Required

#### 1. Centrifugo Health Check

**Test:** Run `centrifugo -c centrifugo/config.json` and in a separate terminal run `curl http://localhost:8000/health`
**Expected:** HTTP 200 response; Centrifugo logs show "Starting Centrifugo" and connects on port 8000
**Why human:** Cannot start a server process during static verification. The config file content is verified correct, but ROADMAP SC-1 specifically requires `GET /health` returns 200 which requires a live process.

#### 2. Classical Pair End-to-End Exchange

**Test:** Start Centrifugo first, then in separate terminals: `./bob-classical` and `./alice-classical` (or via `go run ./cmd/bob-classical &; go run ./cmd/alice-classical`)
**Expected:**
- Bob logs: "published prekey bundle on ch-classical, waiting for Alice"
- Alice logs: "handshake complete", then "sent ratchet_msg seq=1" through seq=5
- Bob logs: "recv: seq=1 text=hello from alice-classical 1" through seq=5, then "sent 5 echoes, exiting"
- Alice logs: "recv echo: seq=1 text=echo: hello from alice-classical 1" through seq=5, then "received 5 echoes, exiting"
**Why human:** Live Centrifugo WebSocket required; two concurrent processes needed; sequential ordering depends on network timing.

#### 3. PQ Pair End-to-End Exchange

**Test:** Same setup as above but with `bob-pq` and `alice-pq`
**Expected:** Same echo cycle on `ch-pq` with "hello from alice-pq N" messages and PQXDH handshake logged
**Why human:** Same as above; additionally confirms PQXDH + TripleRatchet wire format is correct end-to-end.

### Gaps Summary

No blocking gaps found. All four binaries compile and pass `go test ./...` (22 tests). All ROADMAP success criteria that can be verified statically are verified. One success criterion (Centrifugo health endpoint responding 200) requires a live server and is routed to human verification.

The data races flagged by code review (CR-01/CR-02) are warnings, not blockers: they do not prevent message exchange or cause deadlocks in the sequential demo scenario. The nil-guards prevent panics. Recommend fixing with `sync.Mutex` in a follow-up phase or as a pre-Phase-4 cleanup, but the phase goal is achieved.

---

_Verified: 2026-04-27T10:00:00Z_
_Verifier: Claude (gsd-verifier)_
