---
phase: 02-pq-session
plan: 03
subsystem: cryptography
tags: [go, pqxdh, ml-kem-768, triple-ratchet, session-facade, acceptance-tests]

# Dependency graph
requires:
  - phase: 02-pq-session
    plan: 01
    provides: internal/pq/pq.go scaffold with stub function bodies
  - phase: 02-pq-session
    plan: 02
    provides: Full MLKEMProvider implementation as scka.Provider

provides:
  - Full NewResponderBundle, InitiatorHandshake, ResponderHandshake implementation
  - Session.Encrypt and Session.Decrypt wiring result.AD through every message
  - Session.Close() returning error from tr.Close()
  - Passing PQ-01, PQ-03, PQ-04 acceptance tests
  - All 5 pq tests green; full suite green (16 tests, 10 packages)

affects:
  - 03-centrifugo (imports internal/pq.Session, InitiatorHandshake, ResponderHandshake)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "pqxdh.SendHandshake returns (HandshakeResult, InitialMessage, error)"
    - "pqxdh.ReceiveHandshake requires *KEMPreKey via kemOPK.DecapsKey() — not *KEMOneTimePreKey"
    - "doubleratchet.InitAliceTripleRatchet uses bundle.SignedPreKey as bobDRPK"
    - "doubleratchet.InitBobTripleRatchet uses spk.PrivateKey/PublicKey as DR keypair (mirrors classical)"
    - "TripleRatchetSession.Encrypt returns VALUE TripleRatchetMessage — facade wraps in pointer: &msg"
    - "TripleRatchetSession.Decrypt takes VALUE — facade dereferences: *msg"
    - "Session.Close() returns error (TripleRatchetSession.Close returns error, unlike doubleratchet.Session)"
    - "PQXDH.MLKEM768 passed explicitly to GenerateKEMSPK and GenerateKEMOPK — library default is MLKEM1024"

key-files:
  created: []
  modified:
    - internal/pq/pq.go
    - internal/pq/pq_test.go

key-decisions:
  - "Bob's DR keypair must be spk.PrivateKey/PublicKey (not separately generated drPriv/drPub) so Alice's bundle.SignedPreKey matches Bob's DR public key"
  - "Session.Close() signature changed from void to error — TripleRatchetSession.Close() returns error unlike doubleratchet.Session.Close()"
  - "drPriv/drPub fields in ResponderKeys are generated but unused — SPK serves as both PQXDH signed prekey and initial DR keypair (mirrors classical pattern)"

# Metrics
duration: ~8min
completed: 2026-04-27
---

# Phase 2 Plan 03: PQXDH Handshake + Triple Ratchet Session Facade Summary

**Full PQXDH handshake wiring with ML-KEM-768 OPKs and Triple Ratchet session facade — NewResponderBundle/InitiatorHandshake/ResponderHandshake fully implemented, Encrypt/Decrypt threading HandshakeResult.AD, all 5 PQ acceptance tests passing**

## Performance

- **Duration:** ~8 min
- **Completed:** 2026-04-27
- **Tasks:** 3 (Tasks 1+2 implemented together in pq.go; Task 3 activated test stubs)
- **Files modified:** 2

## Accomplishments

- Implemented `NewResponderBundle`: generates IK, SPK (XEdDSA-signed), OPK, KEMSPK (MLKEM768), KEMOPK (MLKEM768), and assembles `pqxdh.PrekeyBundle` with explicit `pqxdh.MLKEM768` param
- Implemented `InitiatorHandshake`: generates Alice's IK, calls `pqxdh.SendHandshake`, wires `MLKEMProvider`, calls `InitAliceTripleRatchet` with `bundle.SignedPreKey` as `bobDRPK`
- Implemented `ResponderHandshake`: calls `kemOPK.DecapsKey()` to get `*KEMPreKey`, calls `pqxdh.ReceiveHandshake`, wires `MLKEMProvider`, calls `InitBobTripleRatchet` with SPK as DR keypair
- Implemented `Session.Encrypt`: wraps `TripleRatchetMessage` value in pointer (`&msg`)
- Implemented `Session.Decrypt`: dereferences pointer before passing to `tr.Decrypt` (`*msg`), nil-guard
- Implemented `Session.Close()`: returns `tr.Close()` error (signature updated from void)
- Activated 3 test stubs by removing `t.Skip(...)` lines: TestPQXDHHandshake, TestPQSession, TestPQWireOverhead

## Task Commits

1. **Task 1+2: Implement pq.go handshake + session methods** — `c090960` (feat)
2. **Task 1+2 bugfix: Use spk keypair as DR keypair** — `885fef6` (fix)
3. **Task 3: Activate PQ acceptance tests** — `e911a44` (test)

## Files Modified

- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/pq.go` — All 5 stub functions fully implemented (~101 net insertions)
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/pq_test.go` — 3 t.Skip stubs removed (6 net deletions)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] DR keypair mismatch: plan used separately generated drPriv/drPub for Bob's DR keypair**
- **Found during:** Task 3 verification — `TestPQSession` failed with `authentication failure`
- **Issue:** Plan's `ResponderHandshake` passed `priv.drPriv/drPub` (separately generated via `doubleratchet.GenerateKeyPair()`) as Bob's DR keypair to `InitBobTripleRatchet`. But Alice's `InitiatorHandshake` uses `bundle.SignedPreKey` (the SPK public key) as `bobDRPK`. These are different keys — authentication fails on every message.
- **Root cause:** Plan's `NewResponderBundle` generates a separate `drPriv/drPub` keypair but never includes `drPub` in the `pqxdh.PrekeyBundle` for Alice to use. The `pqxdh.PrekeyBundle` struct has no dedicated DR field.
- **Fix:** Changed `bobDRKP` in `ResponderHandshake` to use `priv.spk.PrivateKey/PublicKey` — the SPK that Alice already has via `bundle.SignedPreKey`. This mirrors the classical implementation exactly (`internal/classical/classical.go` uses `priv.spk.PrivateKey/PublicKey` as `bobRatchetKP`).
- **Files modified:** `internal/pq/pq.go`
- **Commit:** `885fef6`

**2. [Rule 2 - Missing functionality] Session.Close() signature upgraded from void to error**
- **Found during:** Task 2 implementation
- **Issue:** Current stub `func (s *Session) Close()` returns nothing, but `doubleratchet.TripleRatchetSession.Close()` returns `error`. Plan specifies `func (s *Session) Close() error`. Ignoring the error would silently swallow key-material-zeroing failures.
- **Fix:** Changed signature to `func (s *Session) Close() error` and propagates `tr.Close()` error.
- **Files modified:** `internal/pq/pq.go`
- **Commit:** `c090960`

## Known Stubs

None — all functions in `internal/pq/pq.go` fully implemented. All 5 tests in `pq_test.go` active and passing.

## Threat Flags

None — no new network endpoints, auth paths, or file access patterns introduced. All crypto delegates to `go-doubleratchet v0.0.2` and `crypto/mlkem` as intended per threat model T-2-01 through T-2-05.

## Self-Check: PASSED

- `internal/pq/pq.go` contains `pqxdh.SendHandshake` — FOUND
- `internal/pq/pq.go` contains `pqxdh.ReceiveHandshake` — FOUND
- `internal/pq/pq.go` contains `doubleratchet.InitAliceTripleRatchet` — FOUND
- `internal/pq/pq.go` contains `doubleratchet.InitBobTripleRatchet` — FOUND
- `internal/pq/pq.go` contains `pqxdh.MLKEM768` — FOUND
- `internal/pq/pq.go` contains `.DecapsKey()` — FOUND
- `internal/pq/pq.go` contains `return &msg, nil` — FOUND
- `internal/pq/pq.go` contains `s.tr.Decrypt(*msg, s.ad)` — FOUND
- `internal/pq/pq_test.go` does NOT contain `t.Skip` for TestPQXDHHandshake, TestPQSession, TestPQWireOverhead — CONFIRMED
- Commits c090960, 885fef6, e911a44 exist — CONFIRMED
- `go test ./internal/pq/... -v` — 5 PASS, 0 FAIL
- `go test ./...` — 16 PASS across 10 packages, 0 FAIL
