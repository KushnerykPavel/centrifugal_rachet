---
phase: 02-pq-session
verified: 2026-04-27T14:00:00Z
status: passed
score: 4/4 roadmap success criteria verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs — KEM ratchet epoch now contributes genuine shared entropy via correct two-round Encapsulate/Decapsulate protocol (CR-01 and CR-02 resolved)"
  gaps_remaining: []
  regressions: []
---

# Phase 2: PQ Session Verification Report (Re-verification)

**Phase Goal:** A working post-quantum encrypted chat session exists using PQXDH + Triple Ratchet, verified by unit tests confirming PQ wire overhead is captured
**Verified:** 2026-04-27T14:00:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (CR-01 and CR-02)

---

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs via HKDF in Signal-spec KDF input order — AND the ML-KEM ciphertext is correctly transmitted and decapsulated so both peers derive the same KEM shared secret | VERIFIED | `provider.go` line 87: `ss, ct := ek.Encapsulate()` — ciphertext captured and returned as `msg`. `provider.go` line 133: `dk.Decapsulate(msg)` called in the `mlkem768CiphertextSize` branch. `TestMLKEMProviderKEMProtocol` directly asserts `require.Equal(t, bobOutputKey2, aliceOutputKey2)` — PASSES. |
| 2 | `MLKEMProvider.Snapshot()` performs a deep copy — mutating the original does not affect the snapshot | VERIFIED | `provider.go` line 160: `append([]byte(nil), p.latestPeerEncapKey...)`. `[64]byte` decapSeed copies by value. `TestMLKEMProviderSnapshot` PASSES (regression-checked). |
| 3 | Unit test passes: `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` over a TripleRatchetSession | VERIFIED | `TestPQSession` PASSES. Round-trip confirmed over full Triple Ratchet session with genuine KEM ratchet contribution. |
| 4 | Unit test confirms `len(serialized TripleRatchetMessage) > len(serialized *Message)` for identical plaintext, proving PQ overhead is captured | VERIFIED | `TestPQWireOverhead` PASSES. `require.Greater(t, len(pqJSON), len(classicalJSON))` asserts pass. The first PQ send now emits the 1184-byte encap key (Round 1 announcer path) in `SCKAHeader.Msg`; subsequent sends after a full round will emit the 1088-byte ciphertext — both dominate classical message size. |

**Score: 4/4 roadmap truths verified**

---

### CR-01 and CR-02 Gap Closure — Detailed Confirmation

**CR-01 (resolved):** In the prior implementation, `Send()` called `ek.Encapsulate()` and discarded the ciphertext with `_`. Current code at line 87 captures `ss, ct := ek.Encapsulate()` and returns `ct` as `msg` (line 91). The ciphertext is now transmitted to the peer.

**CR-02 (resolved):** In the prior implementation, `dk.Decapsulate()` was never called anywhere in the codebase. Current `Receive()` dispatches on `len(msg)`: when `len(msg) == mlkem768CiphertextSize` (1088), line 133 calls `dk.Decapsulate(msg)` using the stored `p.decapSeed` to reconstruct the decapsulation key. Both sides now derive the same shared secret.

**New acceptance test (`TestMLKEMProviderKEMProtocol`)** directly exercises the two-round protocol at the provider level, independent of the Triple Ratchet layer. It asserts byte-equality of the output keys and matching key epochs. This test PASSES.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/pq/pq.go` | Full implementation: NewResponderBundle, InitiatorHandshake, ResponderHandshake, Encrypt, Decrypt, Close; min 120 lines | VERIFIED | 167 lines. All functions present. Dead `drPriv`/`drPub` fields removed from `ResponderKeys` (WR-01 from prior report, resolved). |
| `internal/pq/provider.go` | Full MLKEMProvider implementation of scka.Provider; min 100 lines; deep-copy guard; correct Encapsulate/Decapsulate pairing | VERIFIED | 197 lines. All 7 scka.Provider methods implemented. Two-round protocol correct: encapsulator path returns ciphertext; announcer path decapsulates received ciphertext. |
| `internal/pq/pq_test.go` | Activated test bodies for PQ-01, PQ-02, PQ-03, PQ-04; no t.Skip; TestMLKEMProviderKEMProtocol present; contains json.Marshal | VERIFIED | 186 lines. No `t.Skip`. Contains `TestPQXDHHandshake`, `TestMLKEMProviderSnapshot`, `TestMLKEMProviderClose`, `TestMLKEMProviderKEMProtocol`, `TestPQSession`, `TestPQWireOverhead`. All pass. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/pq/pq.go` | `go-doubleratchet/pqxdh` | `pqxdh.SendHandshake` | WIRED | Line 91: `pqxdh.SendHandshake(aliceIK, bundle)` |
| `internal/pq/pq.go` | `go-doubleratchet/pqxdh` | `pqxdh.ReceiveHandshake` | WIRED | Line 114: `pqxdh.ReceiveHandshake(...)` |
| `internal/pq/pq.go` | `go-doubleratchet` | `doubleratchet.InitAliceTripleRatchet` | WIRED | Line 97 |
| `internal/pq/pq.go` | `go-doubleratchet` | `doubleratchet.InitBobTripleRatchet` | WIRED | Line 122 |
| `internal/pq/provider.go` | `crypto/mlkem` | `mlkem.NewDecapsulationKey768` | WIRED | Lines 100, 129 |
| `internal/pq/provider.go` | `crypto/mlkem` | `ek.Encapsulate()` returns `(ss, ct)` — ct transmitted | WIRED | Line 87: `ss, ct := ek.Encapsulate()` |
| `internal/pq/provider.go` | `crypto/mlkem` | `dk.Decapsulate(msg)` called on received ciphertext | WIRED | Line 133 |
| `internal/pq/pq_test.go` | `encoding/json` | `json.Marshal` | WIRED | Lines 178, 180 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `pq.go` Session.Encrypt/Decrypt | `s.ad` | `pqxdh.HandshakeResult.AD` — genuine library output | YES | FLOWING |
| `pq.go` Session.Encrypt/Decrypt | `s.tr` (TripleRatchetSession) | `doubleratchet.InitAlice/BobTripleRatchet` with `result.RootKey[:]` | YES | FLOWING |
| `provider.go` Send() encapsulator path | `outputKey` | `ss` from `ek.Encapsulate()` against peer's real encap key; `ct` transmitted to peer | YES — genuine KEM shared secret | FLOWING |
| `provider.go` Receive() decapsulator path | `outputKey` | `ss` from `dk.Decapsulate(msg)` — reconstructs `dk` from stored seed, decapsulates peer's ciphertext | YES — same shared secret as encapsulator | FLOWING |

### Behavioral Spot-Checks

All 6 tests in `internal/pq` package pass. Full suite (17 tests across 10 packages) passes with no regressions.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TestMLKEMProviderKEMProtocol: bobOutputKey == aliceOutputKey (CR-01/CR-02 regression) | `go test ./internal/pq/... -run TestMLKEMProviderKEMProtocol -v` | PASS | PASS |
| TestPQXDHHandshake: alice.RootKey == bob.RootKey | `go test ./internal/pq/... -run TestPQXDHHandshake` | PASS | PASS |
| TestMLKEMProviderSnapshot: deep copy holds | `go test ./internal/pq/... -run TestMLKEMProviderSnapshot` | PASS | PASS |
| TestPQSession: round-trip decryption | `go test ./internal/pq/... -run TestPQSession` | PASS | PASS |
| TestPQWireOverhead: pqJSON > classicalJSON | `go test ./internal/pq/... -run TestPQWireOverhead` | PASS | PASS |
| Full suite: no regressions | `go test ./...` | 17 PASS, 0 FAIL | PASS |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|---------|
| PQ-01 | `internal/pq` implements PQXDH key agreement combining ML-KEM-768 and X25519, joined via HKDF matching Signal PQXDH spec KDF input order | SATISFIED | `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` handle HKDF combination. `TestPQXDHHandshake` confirms `alice.RootKey == bob.RootKey`. KEM ratchet epoch contribution is now genuine (CR-01/CR-02 fixed). `TestMLKEMProviderKEMProtocol` confirms shared KEM secret. |
| PQ-02 | `MLKEMProvider` implementing `scka.Provider`; `Snapshot()` performs a deep copy | SATISFIED | All 7 interface methods present. `append([]byte(nil), ...)` deep-copy confirmed. `TestMLKEMProviderSnapshot` PASSES. |
| PQ-03 | `TripleRatchetSession` exposed via `Encrypt`/`Decrypt` | SATISFIED | `TestPQSession` PASSES. `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` confirmed. |
| PQ-04 | Unit test confirms PQ wire size > classical wire size | SATISFIED | `TestPQWireOverhead` PASSES. `require.Greater(t, len(pqJSON), len(classicalJSON))` confirmed. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/pq/provider.go` | 19, 34 | `pendingPeerCiphertext []byte` declared in struct and snapshot, deep-copied in `Snapshot()` (line 163), restored in `Restore()` (line 176), zeroed in `Close()` (lines 191-194), but never written anywhere — dead key-material-shaped state | Warning | No correctness impact. Misleading to future readers; inflates Close() zeroing scope. New WR-01 raised in 02-REVIEW.md; not blocking for phase goal. |
| `internal/pq/provider.go` | 42, 54 | `InitInitiator` and `InitResponder` ignore `sk` parameter | Info | No correctness impact. Silently discards caller-provided seed. Documented in IN-01 of 02-REVIEW.md. |
| `internal/pq/provider.go` | 169-172 | `Restore()` silently no-ops on type mismatch | Info | Potential silent state failure during rollback with wrong snapshot type. Documented in IN-02 of 02-REVIEW.md. |

### Human Verification Required

None. All assertions are programmatically verifiable via unit tests and code inspection.

---

## Summary

**Re-verification result: PASSED**

Both critical gaps from the prior verification are confirmed closed:

- **CR-01** (KEM ciphertext discarded): `Send()` now captures `ss, ct := ek.Encapsulate()` and returns `ct` as `msg`. The ciphertext crosses the wire.
- **CR-02** (Decapsulate never called): `Receive()` now dispatches on message length and calls `dk.Decapsulate(msg)` when a ciphertext arrives, recovering the shared secret from the stored seed.

A new test `TestMLKEMProviderKEMProtocol` directly verifies the two-round protocol: Alice announces her encap key (1184 bytes), Bob encapsulates and returns the ciphertext (1088 bytes), both sides independently derive the same 32-byte shared secret. This test passes.

No regressions in the existing suite (17/17 tests pass). The `pendingPeerCiphertext` dead field (new WR-01 from the gap-closure review) is a warning-level anti-pattern with no correctness impact and does not block phase goal achievement.

All four ROADMAP Success Criteria are now fully met. The phase goal is achieved.

---

_Verified: 2026-04-27T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Re-verification: Yes — gap closure for CR-01 and CR-02_
