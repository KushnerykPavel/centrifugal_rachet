---
phase: 01-foundation-classical-session
plan: 04
subsystem: crypto
tags: [x3dh, double-ratchet, go-doubleratchet, e2ee, tdd]

requires:
  - phase: 01-foundation-classical-session
    provides: go.mod with go-doubleratchet v0.0.2 dependency

provides:
  - internal/classical Session wrapper with X3DH handshake + DR encrypt/decrypt
  - PrekeyBundle, ResponderKeys, InitialMessage, Message types
  - NewResponderBundle, InitiatorHandshake, ResponderHandshake functions
  - Session.Encrypt, Session.Decrypt, Session.Close methods
  - Session.RootKey [32]byte field for handshake verification

affects: [03-centrifugo-integration, 04-observability]

tech-stack:
  added: [filippo.io/edwards25519 v1.1.0 (transitive via x3dh)]
  patterns: [x3dh.SendHandshake/ReceiveHandshake, doubleratchet.InitAlice/InitBob, AD threading, nil-message guard]

key-files:
  created: [internal/classical/classical.go, internal/classical/classical_test.go]
  modified: [go.mod, go.sum]

key-decisions:
  - "Session.RootKey exported as [32]byte copy of HandshakeResult.SharedSecret — underlying DR session has unexported rk"
  - "Encrypt/Decrypt wrap AD threading internally so callers never pass nil ad"
  - "Decrypt(nil) returns explicit error instead of panic — tested in TestClassicalSession_ErrorOnNilMsg"
  - "Bob's SPK serves as initial DR ratchet key pair per Signal convention (verified from x3dh_test.go)"

patterns-established:
  - "X3DH handshake flow: NewResponderBundle → InitiatorHandshake → ResponderHandshake"
  - "DR session init: InitAlice(sharedSecret[:], bobSPK.PublicKey, nil) / InitBob(sharedSecret[:], keyPair, nil)"
  - "Message wrapping: classical.Message{DR: doubleratchet.Message} for future JSON serialization"
  - "AD threading: stored in Session.ad from HandshakeResult.AD, passed on every Encrypt/Decrypt"

requirements-completed: [CLASS-01, CLASS-02, CLASS-03]

duration: 5min
completed: 2026-04-27
---

# Phase 1 Plan 4: Classical Session Summary

**X3DH + Double Ratchet session package wrapping go-doubleratchet v0.0.2 with RootKey equality, bidirectional encrypt/decrypt, and nil-message guard — TDD verified**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-27T11:09:24Z
- **Completed:** 2026-04-27T11:14:23Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 4

## Accomplishments
- X3DH handshake produces matching SharedSecret on Alice and Bob sides (RootKey equality)
- Alice→Bob and Bob→Alice encrypt/decrypt roundtrip confirmed with exact byte comparison
- Decrypt(nil) returns error instead of panicking
- All 3 tests pass with -race flag; full internal/ suite green

## Task Commits

TDD cycle produced these commits:

1. **RED: TestClassicalSession failing tests** - `615bb09` (test)
2. **GREEN: Implement classical.go** - `4d278c9` (feat)
3. **Dependency update** - `96cd698` (chore)

## Files Created/Modified
- `internal/classical/classical.go` - Session wrapper, PrekeyBundle, ResponderKeys, X3DH handshake functions, Encrypt/Decrypt/Close
- `internal/classical/classical_test.go` - 3 tests: TestClassicalSession, TestClassicalSession_BidirectionalExchange, TestClassicalSession_ErrorOnNilMsg
- `go.mod` - Added filippo.io/edwards25519 v1.1.0 transitive dep
- `go.sum` - Checksums for new transitive deps

## Decisions Made
- **Session.RootKey as exported [32]byte**: The underlying `doubleratchet.Session.rk` is unexported. Copying `HandshakeResult.SharedSecret` into the wrapper struct gives CLASS-03 a field to compare while keeping the DR session opaque.
- **AD threading internal**: Callers of `Encrypt`/`Decrypt` never need to manage associated data — it's stored from `HandshakeResult.AD` and passed automatically. This prevents the silent security degradation of passing `nil` ad.
- **nil-message guard**: `Decrypt(nil)` returns an explicit error ("classical: Decrypt: nil message") instead of passing nil to the DR session, which would panic.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Missing transitive dependency after go mod tidy**
- **Found during:** Task 1 (GREEN phase - after creating classical.go)
- **Issue:** Build failed with "missing go.sum entry" for `github.com/KushnerykPavel/go-doubleratchet/internal/ecutil` — the x3dh subpackage imports filippo.io/edwards25519 which wasn't in go.sum yet
- **Fix:** Ran `go mod tidy` which added `filippo.io/edwards25519 v1.1.0` as a transitive dep
- **Files modified:** go.mod, go.sum
- **Verification:** `go test ./internal/classical/... -race` passes after tidy
- **Committed in:** 96cd698 (chore commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Dependency update necessary for build. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Classical session package complete with X3DH + Double Ratchet verified end-to-end
- All Phase 1 plans (01-01 through 01-04) now complete
- Ready for Phase 2 (PQ protocol) which follows the same pattern with PQXDH + Triple Ratchet
- internal/classical types (PrekeyBundle, InitialMessage, Message) are ready for Centrifugo JSON serialization in Phase 3

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| RED  | 615bb09 (test) | ✓ Tests written before implementation |
| GREEN | 4d278c9 (feat) | ✓ All tests pass after implementation |
| REFACTOR | — | Skipped — no cleanup needed |

---
*Phase: 01-foundation-classical-session*
*Completed: 2026-04-27*

## Self-Check: PASSED

All files verified present:
- internal/classical/classical.go ✓
- internal/classical/classical_test.go ✓
- 01-04-SUMMARY.md ✓

Commits verified in git log:
- 615bb09 (test) ✓
- 4d278c9 (feat) ✓
- 96cd698 (chore) ✓
