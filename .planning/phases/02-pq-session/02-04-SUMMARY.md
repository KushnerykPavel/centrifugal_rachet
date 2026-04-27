---
phase: 02-pq-session
plan: "04"
subsystem: crypto
tags: [mlkem, ml-kem-768, kem, post-quantum, triple-ratchet, scka]

requires:
  - phase: 02-pq-session
    provides: MLKEMProvider skeleton and PQXDH handshake (02-02, 02-03)

provides:
  - Correct two-round ML-KEM-768 KEM protocol in MLKEMProvider (Encapsulate/Decapsulate)
  - TestMLKEMProviderKEMProtocol proving byte-equal shared secrets on both peers
  - Removal of dead drPriv/drPub fields from ResponderKeys

affects:
  - Phase 3 (metrics/wire-size): KEM ciphertext now travels the wire (1088 bytes)
  - Phase 4 (Grafana dashboard): genuine KEM shared entropy feeds Triple Ratchet

tech-stack:
  added: []
  patterns:
    - "Two-round KEM protocol: announcer emits encap key (1184B), encapsulator returns ciphertext (1088B), both derive same ss[:32]"
    - "Receive() dispatch by message length — no flags or out-of-band state"
    - "decapSeed stored across rounds to reconstruct DecapsulationKey768 on ciphertext receipt"

key-files:
  created: []
  modified:
    - internal/pq/provider.go
    - internal/pq/pq.go
    - internal/pq/pq_test.go

key-decisions:
  - "Dispatch in Receive() purely by message length (1184 vs 1088) — no separate field or flag needed"
  - "Ciphertext emitted as the entire msg in Round 2 Send() — size difference unambiguously identifies message type"
  - "decapSeed stored from Round 1 and reused in Round 2 Receive() — no separate pendingPeerCiphertext field needed at runtime"

patterns-established:
  - "MLKEMProvider.Send() checks latestPeerEncapKey first — encapsulator path takes priority over announcer path"
  - "pendingPeerCiphertext field kept in struct and snapshot for future use (currently unused at runtime)"

requirements-completed:
  - PQ-01

duration: 12min
completed: 2026-04-27
---

# Phase 2 Plan 04: MLKEMProvider KEM Protocol Fix Summary

**ML-KEM-768 two-round KEM protocol fixed: announcer emits 1184-byte encap key, encapsulator returns 1088-byte ciphertext via dk.Decapsulate(), both peers derive byte-equal 32-byte shared secrets**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-04-27T10:15:00Z
- **Completed:** 2026-04-27T10:27:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Rewrote MLKEMProvider.Send() and Receive() to implement the genuine two-round KEM protocol (D-06): announcer path emits fresh encap key, encapsulator path encapsulates and transmits ciphertext
- Eliminated the `ss, _ := ek.Encapsulate()` ciphertext discard bug — ciphertext (1088 bytes) now flows over the wire and is decapsulated by the peer
- dk.Decapsulate() called for the first time in the codebase, recovering the same shared secret as ek.Encapsulate() produced
- Added mlkem768EncapKeySize (1184) and mlkem768CiphertextSize (1088) constants; Receive() dispatches by message length
- Added pendingPeerCiphertext field to MLKEMProvider and snapshot; zeroed in Close() (T-02gc-02)
- Removed dead drPriv/drPub fields from ResponderKeys and doubleratchet.GenerateKeyPair() call from NewResponderBundle() (WR-01)
- Added TestMLKEMProviderKEMProtocol proving byte-equal shared secrets after the two-round exchange

## Task Commits

Each task was committed atomically:

1. **Task 1: Rewrite MLKEMProvider with correct two-round KEM protocol** - `84444e7` (feat)
2. **Task 2: Remove dead drPriv/drPub fields and add KEM equality test** - `4e2d546` (feat)

**Plan metadata:** see final docs commit below

## Files Created/Modified

- `internal/pq/provider.go` - Rewrote Send()/Receive() with correct two-round protocol, added size constants and pendingPeerCiphertext
- `internal/pq/pq.go` - Removed drPriv, drPub fields from ResponderKeys and GenerateKeyPair() call
- `internal/pq/pq_test.go` - Added TestMLKEMProviderKEMProtocol with Round 1 / Round 2 assertions and byte-equality check

## Decisions Made

- Dispatch in Receive() is purely by message length (1184 vs 1088) — clean, no flags or extra state needed
- Round 2 Send() emits ONLY the ciphertext (1088B) — the size difference is sufficient to distinguish from an encap key
- decapSeed is preserved across rounds in the struct; Round 2 Receive() reconstructs DecapsulationKey768 from it

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- ROADMAP Phase 2 Success Criterion 1 is now fully met: ML-KEM-768 contributes genuine shared entropy to the Triple Ratchet KEM epoch
- Phase 3 (metrics/wire-size instrumentation) can proceed — the KEM ciphertext now actually travels the wire as a 1088-byte payload
- All 17 tests pass (16 pre-existing + TestMLKEMProviderKEMProtocol)

## Self-Check

- [x] `internal/pq/provider.go` exists and contains `dk.Decapsulate`
- [x] `internal/pq/pq.go` exists with no drPriv/drPub
- [x] `internal/pq/pq_test.go` contains `TestMLKEMProviderKEMProtocol`
- [x] Commits `84444e7` and `4e2d546` exist in git log
- [x] `go test ./... -count=1` — 17 tests pass

## Self-Check: PASSED

---
*Phase: 02-pq-session*
*Completed: 2026-04-27*
