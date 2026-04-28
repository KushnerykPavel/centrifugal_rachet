---
phase: 04-prometheus-metrics
plan: "01"
subsystem: cmd/alice-classical, cmd/bob-classical
tags: [metrics, prometheus, sync, instrumentation]
dependency_graph:
  requires: [internal/metrics/metrics.go, internal/classical, internal/protocol]
  provides: [instrumented alice-classical binary, instrumented bob-classical binary]
  affects: [cmd/alice-classical/main.go, cmd/bob-classical/main.go]
tech_stack:
  added: []
  patterns: [sync.Mutex session guard, METRICS_PORT env-var, histogram Observe pattern]
key_files:
  created: []
  modified:
    - cmd/alice-classical/main.go
    - cmd/bob-classical/main.go
decisions:
  - "HandshakeDurationSeconds timer starts before InitiatorHandshake in alice and at TypeInitialMsg arrival (before unmarshal) in bob — captures full responder processing time"
  - "sess nil-check moved to mu.Lock/Unlock read-guard to prevent data race while keeping early-return behavior"
  - "MessageWireBytes.Observe uses float64(len(data)) on receive (full outer envelope wire bytes) and float64(len(raw)) on send (after MarshalEnvelope)"
metrics:
  duration: "91 seconds"
  completed: "2026-04-27"
  tasks_completed: 3
  files_modified: 2
---

# Phase 4 Plan 01: Instrument Classical Pair Binaries Summary

Classical pair binaries (alice-classical, bob-classical) instrumented with all four Prometheus histograms, METRICS_PORT env-var, and sync.Mutex session guard.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Instrument alice-classical/main.go | eb7c4fd | cmd/alice-classical/main.go |
| 2 | Instrument bob-classical/main.go | 701c2c0 | cmd/bob-classical/main.go |
| 3 | Verify go build and go test pass | (no commit — verification only) | — |

## What Was Built

Both classical-pair binaries now:

1. Read `METRICS_PORT` env var (defaults: alice=9091, bob=9092) and serve `/metrics` on that port.
2. Declare `mu sync.Mutex` alongside `sess`; every read and write of `sess` and every `Encrypt`/`Decrypt` call is bracketed by `mu.Lock()`/`mu.Unlock()`.
3. Observe `metrics.HandshakeDurationSeconds` once per handshake.
4. Observe `metrics.EncryptDurationSeconds` once per `Encrypt` call.
5. Observe `metrics.DecryptDurationSeconds` once per `Decrypt` call.
6. Observe `metrics.MessageWireBytes` once per ratchet_msg wire event (receive path uses `len(data)`, send path uses `len(raw)` after `MarshalEnvelope`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Added mutex-guarded nil check for sess**

- **Found during:** Task 1 and Task 2
- **Issue:** The plan instructed wrapping `sess == nil` bare checks, but did not explicitly show the mutex-guarded nil read pattern needed to avoid a data race on the nil check itself.
- **Fix:** Replaced `if sess == nil` with `mu.Lock(); sessNil := sess == nil; mu.Unlock()` before acting on the nil value.
- **Files modified:** cmd/alice-classical/main.go, cmd/bob-classical/main.go
- **Commit:** eb7c4fd, 701c2c0

## Known Stubs

None.

## Threat Flags

None — all trust boundary mitigations from the plan's threat model are implemented:

- T-04-01 (Tampering / sess data race): mitigated — sync.Mutex wraps every sess access.
- T-04-02 (unauthenticated /metrics): accepted by design (demo project).
- T-04-03 (port conflict DoS): accepted by design (single-instance demo).

## Self-Check: PASSED

- cmd/alice-classical/main.go — FOUND
- cmd/bob-classical/main.go — FOUND
- Commit eb7c4fd — verified (alice-classical instrumentation)
- Commit 701c2c0 — verified (bob-classical instrumentation)
- go build ./... — PASSED
- go test ./... — PASSED (22 tests, 11 packages)
