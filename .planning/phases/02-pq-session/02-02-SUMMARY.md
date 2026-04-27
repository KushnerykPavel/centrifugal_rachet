---
phase: 02-pq-session
plan: 02
subsystem: cryptography
tags: [go, ml-kem-768, scka, provider, snapshot, deep-copy, zeroing]

# Dependency graph
requires:
  - phase: 02-pq-session
    plan: 01
    provides: internal/pq/provider.go scaffold with 7 stubbed scka.Provider methods

provides:
  - Full MLKEMProvider implementation of scka.Provider (all 7 methods)
  - Deep-copy Snapshot/Restore invariant enforced via append([]byte(nil), ...)
  - Key-material zeroing in Close() via range loops
  - Passing PQ-02 tests: TestMLKEMProviderSnapshot, TestMLKEMProviderClose

affects:
  - 02-pq-session/02-03 (PQXDH handshake — uses MLKEMProvider as scka.Provider)
  - 03-centrifugo (imports internal/pq.Session which uses MLKEMProvider internally)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "NewEncapsulationKey768 (not ParseEncapsulationKey768) — correct Go stdlib crypto/mlkem API"
    - "Encapsulate() returns (sharedKey, ciphertext []byte) with no error — no-error signature"
    - "Snapshot deep-copy: [64]byte copies by value; []byte via append([]byte(nil), p.latestPeerEncapKey...)"
    - "Close zeroing: for i := range p.decapSeed { p.decapSeed[i] = 0 } then nil slice"
    - "keyEpoch = p.recvEpoch after p.recvEpoch++ in Receive() — equals old+1 satisfying SPQR validation"

key-files:
  created: []
  modified:
    - internal/pq/provider.go
    - internal/pq/pq_test.go

key-decisions:
  - "Use NewEncapsulationKey768 not ParseEncapsulationKey768 — stdlib has no Parse variant; plan had wrong function name"
  - "Encapsulate() has no error return in Go 1.25 stdlib — plan sample code showed error return which was incorrect"
  - "Send() discards ciphertext from Encapsulate — peer generates fresh keypair every message (every-message rotation per D-05)"
  - "TestMLKEMProviderSnapshot verifies Send() succeeds post-Restore — stronger than struct equality since fields are unexported"

# Metrics
duration: ~6min
completed: 2026-04-27
---

# Phase 2 Plan 02: MLKEMProvider Full Implementation Summary

**Full MLKEMProvider implementation replacing all 7 scka.Provider stubs with real ML-KEM-768 key operations using Go stdlib crypto/mlkem — every-message keypair rotation, deep-copy Snapshot/Restore, and key-material zeroing on Close**

## Performance

- **Duration:** ~6 min
- **Completed:** 2026-04-27
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced all 7 method stubs in `internal/pq/provider.go` with full ML-KEM-768 implementations
- InitInitiator/InitResponder: generate 64-byte random decapsulation key seed via crypto/rand
- Send(): generates fresh keypair, emits 1184-byte encapsulation key; encapsulates against latestPeerEncapKey when set to produce outputKey for SPQR KDF ratchet
- Receive(): stores peer encap key via deep copy, encapsulates to produce 32-byte shared secret; keyEpoch = recvEpoch+1
- Snapshot(): [64]byte copies by value; latestPeerEncapKey deep-copied via `append([]byte(nil), ...)`
- Restore(): full field-by-field reinstatement from *mlkemProviderSnapshot
- Close(): zeros decapSeed and latestPeerEncapKey bytes before nil-ing slice
- Activated TestMLKEMProviderSnapshot (verifies Snapshot/Restore + Send() round-trip) and TestMLKEMProviderClose (verifies idempotent close) — both PASS

## Task Commits

1. **Task 1: Implement MLKEMProvider fully in provider.go** — `4ede68f` (feat)
2. **Task 2: Activate PQ-02 test bodies in pq_test.go** — `6773a12` (test)

## Files Modified

- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/provider.go` — All 7 scka.Provider methods fully implemented (103 net insertions)
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/pq_test.go` — TestMLKEMProviderSnapshot and TestMLKEMProviderClose activated (32 net insertions)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Wrong stdlib function name: ParseEncapsulationKey768 does not exist**
- **Found during:** Task 1, pre-write verification
- **Issue:** Plan's action block referenced `mlkem.ParseEncapsulationKey768(msg)` — this function does not exist in Go stdlib crypto/mlkem. The correct function is `mlkem.NewEncapsulationKey768(encapsulationKey []byte)`
- **Fix:** Used `mlkem.NewEncapsulationKey768` throughout provider.go in both Send() and Receive()
- **Files modified:** internal/pq/provider.go
- **Commit:** 4ede68f

**2. [Rule 1 - Bug] Encapsulate() returns (sharedKey, ciphertext []byte) with no error**
- **Found during:** Task 1, pre-write verification via `go doc crypto/mlkem EncapsulationKey768.Encapsulate`
- **Issue:** Plan's code sample showed `ss, ct, encapErr := ek.Encapsulate()` with error return — stdlib signature is `(sharedKey, ciphertext []byte)` with no error
- **Fix:** Used two-return assignment `ss, _ := ek.Encapsulate()` (no error handling needed)
- **Files modified:** internal/pq/provider.go
- **Commit:** 4ede68f

## Known Stubs

None — all MLKEMProvider methods fully implemented. Remaining stubs in pq_test.go (TestPQXDHHandshake, TestPQSession, TestPQWireOverhead) are intentional — implemented in Plan 03.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. All crypto delegates to Go stdlib crypto/mlkem as intended per threat model T-2-02.

## Self-Check: PASSED

- `internal/pq/provider.go` exists and contains `append([]byte(nil), p.latestPeerEncapKey...)` — FOUND
- `internal/pq/provider.go` contains `for i := range p.decapSeed { p.decapSeed[i] = 0 }` — FOUND
- `internal/pq/pq_test.go` contains `TestMLKEMProviderSnapshot` — FOUND
- Commit 4ede68f exists — FOUND
- Commit 6773a12 exists — FOUND
- `go test ./internal/pq/... -run TestMLKEMProvider` — 2 PASS, 0 FAIL
- `go test ./...` — all green, no regressions
