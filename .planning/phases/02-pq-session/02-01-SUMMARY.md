---
phase: 02-pq-session
plan: 01
subsystem: cryptography
tags: [go, pqxdh, triple-ratchet, ml-kem-768, scka, go-doubleratchet]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: go.mod with go-doubleratchet v0.0.2 pinned, internal/classical Session facade pattern

provides:
  - internal/pq package scaffold with compilable stubs for all PQ types and functions
  - MLKEMProvider struct implementing scka.Provider interface (all 7 methods stubbed)
  - Session facade with TripleRatchetMessage type alias (D-12) and RootKey exported field
  - Test stubs PQ-01 through PQ-04 that compile and skip safely

affects:
  - 02-pq-session/02-02 (MLKEMProvider implementation — builds on provider.go scaffold)
  - 02-pq-session/02-03 (PQXDH handshake implementation — builds on pq.go scaffold)
  - 03-centrifugo (imports internal/pq.Session)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "type alias re-export: type TripleRatchetMessage = doubleratchet.TripleRatchetMessage (D-12)"
    - "stub pattern: fmt.Errorf (not panic) for safe test invocation of unimplemented functions"
    - "scka.Provider interface: 7-method contract InitInitiator/InitResponder/Send/Receive/Snapshot/Restore/Close"

key-files:
  created:
    - internal/pq/pq.go
    - internal/pq/provider.go
    - internal/pq/pq_test.go
  modified: []

key-decisions:
  - "Wave 0 scaffold: all stubs return fmt.Errorf, not panic — tests can call stubs and get errors safely"
  - "TripleRatchetMessage is a type alias (=), not a new type — enforces D-12 from CONTEXT.md"
  - "Session.tr field is *doubleratchet.TripleRatchetSession, not *doubleratchet.Session — mirrors PQ protocol"
  - "MLKEMProvider.decapSeed is [64]byte (value type) — copies by value in Snapshot without append()"
  - "Test package is pq_test (external), not pq — mirrors classical_test.go pattern"

patterns-established:
  - "PQ package error prefix: fmt.Errorf(\"pq: FunctionName: %w\", err) — consistent with classical: prefix"
  - "Snapshot deep-copy: [64]byte copies by value; []byte requires append([]byte(nil), src...) (Plan 02)"
  - "Test stub order: t.Skip() as FIRST statement, then stub body for compile-time checking"

requirements-completed:
  - PQ-01
  - PQ-02
  - PQ-03
  - PQ-04

# Metrics
duration: 8min
completed: 2026-04-27
---

# Phase 2 Plan 01: PQ Session Scaffold Summary

**Compilable internal/pq package scaffold with Session facade, MLKEMProvider struct, and five skipping PQ test stubs — Wave 0 baseline ensuring go build ./... passes for all subsequent implementation tasks**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-04-27T15:48:00Z
- **Completed:** 2026-04-27T15:56:13Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Created `internal/pq/pq.go` with Session struct (RootKey exported, tr *TripleRatchetSession), ResponderKeys with all 5 PQ key types, TripleRatchetMessage type alias (D-12 compliance), and all handshake function stubs
- Created `internal/pq/provider.go` with MLKEMProvider struct and all 7 scka.Provider method stubs — crypto/mlkem import preserved via reference stub
- Created `internal/pq/pq_test.go` with 5 test stubs (PQ-01 through PQ-04) in external package pq_test that compile and skip cleanly; full `go test ./...` is green

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/pq/pq.go scaffold** - `b6d912a` (feat)
2. **Task 2: Create internal/pq/provider.go scaffold** - `fd1beb1` (feat)
3. **Task 3: Create internal/pq/pq_test.go scaffold** - `dcc770f` (test)

## Files Created/Modified
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/pq.go` - Package declaration, type aliases, Session struct, ResponderKeys struct, function stubs for handshake and Encrypt/Decrypt/Close
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/provider.go` - MLKEMProvider struct, mlkemProviderSnapshot struct, all 7 scka.Provider method stubs
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/pq/pq_test.go` - Test stubs for PQ-01 through PQ-04 (5 test functions, all skipping)

## Decisions Made
- Followed plan exactly as specified — no implementation decisions required for scaffold
- `errors` import preserved in pq.go via nil-message guard in Decrypt stub (not a blank import — actual use)
- `crypto/mlkem` import preserved in provider.go via `_ = mlkem.NewDecapsulationKey768` in InitInitiator stub

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None — all three files compiled on first attempt.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Wave 0 build baseline established: `go build ./...` and `go test ./...` pass cleanly
- Plan 02 can implement MLKEMProvider (provider.go stubs are in place with correct signatures)
- Plan 03 can implement PQXDH handshake (pq.go stubs are in place with correct function signatures)
- No blockers

## Known Stubs

All stubs are intentional — this is a Wave 0 scaffold plan. Each stub documents which plan implements it:

| File | Function/Method | Implementing Plan |
|------|----------------|-------------------|
| pq.go | NewResponderBundle | Plan 03 |
| pq.go | InitiatorHandshake | Plan 03 |
| pq.go | ResponderHandshake | Plan 03 |
| pq.go | Session.Encrypt | Plan 03 |
| pq.go | Session.Decrypt (partial) | Plan 03 |
| provider.go | MLKEMProvider.InitInitiator | Plan 02 |
| provider.go | MLKEMProvider.InitResponder | Plan 02 |
| provider.go | MLKEMProvider.Send | Plan 02 |
| provider.go | MLKEMProvider.Receive | Plan 02 |
| provider.go | MLKEMProvider.Snapshot | Plan 02 |
| provider.go | MLKEMProvider.Restore | Plan 02 |

These stubs do not prevent the plan's goal (compilable build baseline) from being achieved.

---
*Phase: 02-pq-session*
*Completed: 2026-04-27*
