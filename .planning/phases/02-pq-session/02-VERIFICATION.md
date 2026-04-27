---
phase: 02-pq-session
verified: 2026-04-27T00:00:00Z
status: gaps_found
score: 3/4 roadmap success criteria verified
overrides_applied: 0
gaps:
  - truth: "PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs via HKDF in Signal-spec KDF input order — AND the ML-KEM ciphertext is correctly transmitted and decapsulated so both peers derive the same KEM shared secret"
    status: failed
    reason: "CR-01 and CR-02: Both Send() and Receive() call Encapsulate() independently and discard the ciphertext with `_`. In ML-KEM, only one peer encapsulates; the ciphertext must be transmitted so the other peer calls Decapsulate(). Without the ciphertext crossing the wire, each side derives an independent random shared key. The KEM epoch contribution is noise, not a genuine shared secret. Nowhere in the codebase is dk.Decapsulate() ever called. The SCKAHeader.Msg carries the raw encapsulation key (1184 bytes), not a ciphertext — so the peer can never derive the same KEM output."
    artifacts:
      - path: "internal/pq/provider.go"
        issue: "Line 85: ss, _ := ek.Encapsulate() — ciphertext silently dropped in Send(). Line 112: ss, _ := ek.Encapsulate() — ciphertext silently dropped in Receive(). decapSeed is stored (line 70) but dk.Decapsulate() is never invoked anywhere."
    missing:
      - "Receive() must return the KEM ciphertext (not just the encap key) in the next SCKAHeader.Msg, or Send() must encapsulate against the peer encap key and return the ciphertext as msg"
      - "Send() must reconstruct the decapsulation key from p.decapSeed and call dk.Decapsulate(peerCiphertext) to recover the shared secret that matches what the peer computed"
      - "The protocol flow must achieve genuine shared entropy: one side calls Encapsulate() to get (sharedKey, ciphertext); that ciphertext is transmitted; the other side calls Decapsulate(ciphertext) to recover the same sharedKey"
---

# Phase 2: PQ Session Verification Report

**Phase Goal:** A working post-quantum encrypted chat session exists using PQXDH + Triple Ratchet, verified by unit tests confirming PQ wire overhead is captured
**Verified:** 2026-04-27
**Status:** gaps_found
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs via HKDF in Signal-spec KDF input order | PARTIAL | `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` are called correctly; the PQXDH root key is genuine. However the MLKEMProvider that backs the Triple Ratchet's SPQR layer does NOT correctly implement the Encapsulate/Decapsulate pairing — each peer independently calls `Encapsulate()` and discards the ciphertext, so the KEM epoch contribution is not a shared secret. |
| 2 | `MLKEMProvider.Snapshot()` performs a deep copy — mutating the original does not affect the snapshot | VERIFIED | `provider.go:132` uses `append([]byte(nil), p.latestPeerEncapKey...)` for the slice field; `[64]byte` decapSeed copies by value. TestMLKEMProviderSnapshot PASSES. |
| 3 | Unit test passes: `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` over a TripleRatchetSession | VERIFIED | TestPQSession PASSES. The round-trip works because the Triple Ratchet's EC (Double Ratchet) layer provides symmetric encryption regardless of the KEM ratchet epoch. The session is functional; the KEM sub-ratchet does not advance correctly but does not break decryption. |
| 4 | Unit test confirms `len(serialized TripleRatchetMessage) > len(serialized *Message)` for identical plaintext, proving PQ overhead is captured | VERIFIED | TestPQWireOverhead PASSES. `SCKAHeader.Msg` carries the 1184-byte ML-KEM-768 encapsulation key emitted by `Send()`, which dominates the JSON-serialized size. The overhead is real bytes on the wire — it just happens to be an encap key rather than a ciphertext. |

**Score: 3/4 roadmap truths verified** (SC1 partially met — PQXDH root key is correct but KEM ratchet epoch contribution is not a genuine shared secret due to CR-01/CR-02)

---

### Detailed Analysis: The CR-01/CR-02 Bug and Its Impact

**What the bugs are:**

In `provider.go`, `Send()` (line 85) and `Receive()` (line 112) both call `ek.Encapsulate()` which returns `(sharedKey, ciphertext []byte)`. The ciphertext is discarded with `_` in both cases. No call to `dk.Decapsulate(ciphertext)` exists anywhere in the codebase.

In correct ML-KEM:
- Alice calls `ek.Encapsulate()` → gets `(ss_A, ct)`. She keeps `ss_A` and sends `ct` to Bob.
- Bob calls `dk.Decapsulate(ct)` → gets `ss_B`. If Bob's `dk` matches Alice's `ek`, then `ss_A == ss_B`.

In the current implementation:
- Alice calls `ek.Encapsulate()` → gets `(ss_A, ct_A)`, discards `ct_A`, uses `ss_A`.
- Bob calls `ek.Encapsulate()` → gets `(ss_B, ct_B)`, discards `ct_B`, uses `ss_B`.
- `ss_A != ss_B` (independent random shared keys from independent `Encapsulate()` calls).

**Why tests still pass:**

The `TripleRatchetSession` has two layers:
1. The EC (Double Ratchet) layer — provides authenticated encryption using X25519 DH ratchet.
2. The SPQR/SCKA layer (backed by MLKEMProvider) — provides post-quantum key material.

Round-trip encryption/decryption succeeds because the EC layer works correctly. The SPQR layer's KDF contribution simply uses a different random value on each side, but the SPQR layer is designed to be additive — a mismatch causes authentication failures on decryption when the library validates the KEM epoch. The fact that TestPQSession passes means the library either does not validate epoch consistency strictly on the first message, or the `keyEpoch` return values happen to satisfy the SPQR validation constraints even with mismatched shared secrets.

**Why PQ-04 (wire overhead) still passes:**

`Send()` correctly emits `dk.EncapsulationKey().Bytes()` (1184 bytes) as `msg`, which the SPQR layer places in `SCKAHeader.Msg`. This field is present in the serialized `TripleRatchetMessage` regardless of whether the KEM math is correct. So `len(pqJSON) > len(classicalJSON)` holds because the structural overhead is real.

**Impact on the phase goal:**

The phase goal states: *"A working post-quantum encrypted chat session exists using PQXDH + Triple Ratchet, verified by unit tests confirming PQ wire overhead is captured."*

- "Working session" — PARTIAL. Encryption/decryption works. The PQ *contribution to security* from the KEM ratchet is absent (independent random keys on each side rather than a genuine shared KEM secret).
- "Wire overhead captured" — YES. The 1184-byte encap key in SCKAHeader.Msg makes PQ messages measurably larger.

ROADMAP Success Criterion 1 is partially met: the PQXDH handshake root key is genuine (library handles that correctly), but the ongoing KEM ratchet epoch does not contribute real shared entropy. This is a correctness defect in the SPQR layer's MLKEMProvider, not a test gap.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/pq/pq.go` | Full implementation: NewResponderBundle, InitiatorHandshake, ResponderHandshake, Encrypt, Decrypt, Close; min 120 lines | VERIFIED | 176 lines. Contains `pqxdh.SendHandshake`, `pqxdh.ReceiveHandshake`, `doubleratchet.InitAliceTripleRatchet`, `doubleratchet.InitBobTripleRatchet`, `pqxdh.MLKEM768`, `.DecapsKey()`, `s.tr.Encrypt(plaintext, s.ad)`, `s.tr.Decrypt(*msg, s.ad)`. All stubs replaced. |
| `internal/pq/provider.go` | Full MLKEMProvider implementation of scka.Provider; min 100 lines; deep-copy guard | VERIFIED (with known defect) | 161 lines. Contains `append([]byte(nil), p.latestPeerEncapKey...)`. All 7 scka.Provider methods implemented. Defect: ciphertext dropped in `Encapsulate()` calls (CR-01/CR-02). |
| `internal/pq/pq_test.go` | Activated test bodies for PQ-01, PQ-03, PQ-04; no t.Skip; contains json.Marshal | VERIFIED | 138 lines. No `t.Skip` on the three tests. Contains `pq.NewResponderBundle`, `pq.InitiatorHandshake`, `pq.ResponderHandshake`, `require.Equal(t, aliceSess.RootKey, bobSess.RootKey`, `json.Marshal`, `require.Greater`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/pq/pq.go` | `go-doubleratchet/pqxdh` | `pqxdh.SendHandshake` | WIRED | Line 100: `result, initMsg, err := pqxdh.SendHandshake(aliceIK, bundle)` |
| `internal/pq/pq.go` | `go-doubleratchet/pqxdh` | `pqxdh.ReceiveHandshake` | WIRED | Line 123: `result, err := pqxdh.ReceiveHandshake(...)` |
| `internal/pq/pq.go` | `go-doubleratchet` | `doubleratchet.InitAliceTripleRatchet` | WIRED | Line 106 |
| `internal/pq/pq.go` | `go-doubleratchet` | `doubleratchet.InitBobTripleRatchet` | WIRED | Line 131 |
| `internal/pq/provider.go` | `crypto/mlkem` | `mlkem.NewDecapsulationKey768` | WIRED | Line 66 |
| `internal/pq/provider.go` | `crypto/rand` | `rand.Read` | WIRED | Lines 33, 45, 63 |
| `internal/pq/pq_test.go` | `encoding/json` | `json.Marshal` | WIRED | Lines 130, 133 |

### Data-Flow Trace (Level 4)

The PQXDH handshake produces `result.RootKey` (genuine shared secret from library). This flows into `sess.RootKey` and `result.AD` flows into `sess.ad`. Both are threaded through every Encrypt/Decrypt call via `s.tr.Encrypt(plaintext, s.ad)` and `s.tr.Decrypt(*msg, s.ad)`. The EC ratchet layer (Double Ratchet) derives real shared secrets via X25519 DH. The KEM sub-ratchet (MLKEMProvider) does not contribute a genuine shared secret due to CR-01/CR-02.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `pq.go` Session.Encrypt/Decrypt | `s.ad` (associated data) | `pqxdh.HandshakeResult.AD` — genuine library output | YES | FLOWING |
| `pq.go` Session.Encrypt/Decrypt | `s.tr` (TripleRatchetSession) | `doubleratchet.InitAliceTripleRatchet` / `InitBobTripleRatchet` with `result.RootKey[:]` | YES (EC layer) / PARTIAL (KEM layer) | HOLLOW in KEM path |
| `provider.go` Send().outputKey | `ss[:32]` from `Encapsulate()` | Both peers call Encapsulate independently — no ciphertext exchange | NO — independent random keys | DISCONNECTED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TestPQXDHHandshake: alice.RootKey == bob.RootKey | `rtk proxy "go test ./internal/pq/... -run TestPQXDHHandshake -v"` | PASS | PASS |
| TestMLKEMProviderSnapshot: deep copy | `rtk proxy "go test ./internal/pq/... -run TestMLKEMProviderSnapshot -v"` | PASS | PASS |
| TestMLKEMProviderClose: idempotent | `rtk proxy "go test ./internal/pq/... -run TestMLKEMProviderClose -v"` | PASS | PASS |
| TestPQSession: bob.Decrypt(alice.Encrypt(plaintext)) == plaintext | `rtk proxy "go test ./internal/pq/... -run TestPQSession -v"` | PASS | PASS |
| TestPQWireOverhead: len(pqJSON) > len(classicalJSON) | `rtk proxy "go test ./internal/pq/... -run TestPQWireOverhead -v"` | PASS | PASS |
| Full suite | `rtk proxy "go test ./..."` | 16 PASS, 0 FAIL | PASS |
| Build | `rtk proxy "go build ./..."` | exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| PQ-01 | 02-01, 02-03 | `internal/pq` implements PQXDH key agreement combining ML-KEM-768 and X25519, joined via HKDF matching Signal PQXDH spec KDF input order | PARTIAL | PQXDH handshake via library is correct (`pqxdh.SendHandshake`/`ReceiveHandshake`); alice.RootKey == bob.RootKey confirmed by TestPQXDHHandshake. However the ongoing KEM contribution in the Triple Ratchet (MLKEMProvider) does not implement the Encapsulate/Decapsulate pairing correctly (CR-01, CR-02). |
| PQ-02 | 02-01, 02-02 | `MLKEMProvider` implementing `scka.Provider`; `Snapshot()` performs a deep copy | SATISFIED | `provider.go` has all 7 methods. `append([]byte(nil), ...)` deep-copy confirmed. TestMLKEMProviderSnapshot PASSES. |
| PQ-03 | 02-01, 02-03 | `TripleRatchetSession` exposed via `Encrypt`/`Decrypt` | SATISFIED | TestPQSession PASSES. `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` confirmed. |
| PQ-04 | 02-01, 02-03 | Unit test confirms PQ wire size > classical wire size | SATISFIED | TestPQWireOverhead PASSES. `require.Greater(t, len(pqJSON), len(classicalJSON))` asserts verified. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/pq/provider.go` | 85 | `ss, _ := ek.Encapsulate()` — ciphertext silently discarded in Send() | Blocker | KEM epoch does not contribute genuine shared entropy; CR-01 from 02-REVIEW.md |
| `internal/pq/provider.go` | 112 | `ss, _ := ek.Encapsulate()` — ciphertext silently discarded in Receive() | Blocker | Same as above — both sides call Encapsulate independently; CR-01 from 02-REVIEW.md |
| `internal/pq/provider.go` | 66-70 | `decapSeed` stored via `p.decapSeed = newSeed` but `dk.Decapsulate()` is never called | Blocker | The decapsulation half of ML-KEM is dead code; CR-02 from 02-REVIEW.md |
| `internal/pq/pq.go` | 26-27, 63-66, 86-87 | `drPriv`/`drPub` generated but never used | Warning | Dead key material and misleading comment ("included in PrekeyBundle" — it is not); WR-01 from 02-REVIEW.md |
| `internal/pq/provider.go` | 32, 44 | `InitInitiator`/`InitResponder` ignore `sk` parameter | Warning | If sk was intended for deterministic seed derivation it is silently wrong; WR-02 from 02-REVIEW.md |

### Human Verification Required

None. All assertions are programmatically verifiable via unit tests and code inspection.

---

## Gaps Summary

**One gap blocks full goal achievement:**

The ROADMAP Success Criterion 1 requires that the PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs. The library correctly handles the initial PQXDH exchange (root key is genuinely shared). However, the ongoing ML-KEM epoch contribution in the Triple Ratchet's SPQR layer is broken: `MLKEMProvider.Send()` and `Receive()` both call `ek.Encapsulate()` and discard the ciphertext. In correct ML-KEM, only one party encapsulates and transmits the ciphertext; the other party decapsulates. With both parties independently calling `Encapsulate()`, they derive independent random keys that are never reconciled. `dk.Decapsulate()` is never called anywhere.

**Why tests still pass despite this bug:** The Triple Ratchet's EC (Double Ratchet) layer provides correct symmetric encryption independent of the SPQR/KEM layer. The SPQR layer contributes a per-message key epoch, but if both sides derive different KEM-epoch keys, the session can still encrypt/decrypt via the EC chain. TestPQSession passes because the round-trip uses the EC ratchet path. TestPQWireOverhead passes because the SCKAHeader.Msg field carries the 1184-byte encapsulation key regardless of whether the KEM math is correct.

**This is a real defect, not a test gap.** The blog post's central claim — that the PQ session uses ML-KEM-768 for genuine post-quantum security — is not met by the current provider implementation. The fix requires restructuring the Send()/Receive() protocol so that the ciphertext from Encapsulate() is transmitted and decapsulated by the recipient.

---

_Verified: 2026-04-27_
_Verifier: Claude (gsd-verifier)_
