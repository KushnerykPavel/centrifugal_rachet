---
phase: 04-prometheus-metrics
plan: "02"
subsystem: cmd/alice-pq, cmd/bob-pq
tags: [metrics, prometheus, sync, instrumentation, pq]
dependency_graph:
  requires: [internal/metrics/metrics.go, internal/pq, internal/protocol]
  provides: [instrumented alice-pq binary, instrumented bob-pq binary]
  affects: [cmd/alice-pq/main.go, cmd/bob-pq/main.go]
tech_stack:
  added: []
  patterns: [sync.Mutex session guard, METRICS_PORT env-var, histogram Observe pattern]
key_files:
  created: []
  modified:
    - cmd/alice-pq/main.go
    - cmd/bob-pq/main.go
decisions:
  - "HandshakeDurationSeconds timer starts before InitiatorHandshake in alice-pq and at TypeInitialMsg arrival (before unmarshal) in bob-pq — captures full responder processing time"
  - "sess nil-check uses mutex-guarded read pattern (mu.Lock; sessNil := sess == nil; mu.Unlock) to prevent data race on nil check itself"
  - "MessageWireBytes.Observe uses float64(len(data)) on receive (full outer envelope wire bytes) and float64(len(raw)) on send (after MarshalEnvelope)"
metrics:
  duration: "116 seconds"
  completed: "2026-04-28"
  tasks_completed: 2
  files_modified: 2
---

# Phase 4 Plan 02: Instrument PQ Pair Binaries Summary

PQ pair binaries (alice-pq, bob-pq) instrumented with all four Prometheus histograms, METRICS_PORT env-var (defaults 9093/9094), and sync.Mutex session guard — mirroring the classical pair pattern from Plan 04-01 exactly.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Instrument alice-pq/main.go | 724f8cd | cmd/alice-pq/main.go |
| 2 | Instrument bob-pq/main.go | 099745d | cmd/bob-pq/main.go |

## What Was Built

Both PQ-pair binaries now:

1. Read `METRICS_PORT` env var (defaults: alice-pq=9093, bob-pq=9094) and serve `/metrics` on that port.
2. Declare `mu sync.Mutex` alongside `sess`; every read and write of `sess` and every `Encrypt`/`Decrypt` call is bracketed by `mu.Lock()`/`mu.Unlock()`.
3. Use mutex-guarded nil check for `sess` before TypeRatchetMsg processing (`mu.Lock(); sessNil := sess == nil; mu.Unlock()`).
4. Observe `metrics.HandshakeDurationSeconds` once per handshake.
5. Observe `metrics.EncryptDurationSeconds` once per `Encrypt` call.
6. Observe `metrics.DecryptDurationSeconds` once per `Decrypt` call.
7. Observe `metrics.MessageWireBytes` once per ratchet_msg wire event (receive path uses `len(data)`, send path uses `len(raw)` after `MarshalEnvelope`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Added mutex-guarded nil check for sess**

- **Found during:** Task 1 and Task 2
- **Issue:** The plan's action blocks did not show the mutex-guarded nil read pattern for the `sess == nil` check in TypeRatchetMsg. Without it, a goroutine can race on the nil read while another goroutine writes `sess` under the mutex.
- **Fix:** Replaced `if sess == nil` with `mu.Lock(); sessNil := sess == nil; mu.Unlock()` before acting on the nil value — exactly as implemented in the classical pair (Plan 04-01 deviation fix).
- **Files modified:** cmd/alice-pq/main.go, cmd/bob-pq/main.go
- **Commit:** 724f8cd, 099745d

## Known Stubs

None.

## Threat Flags

None — all trust boundary mitigations from the plan's threat model are implemented:

- T-04-04 (Tampering / sess data race): mitigated — sync.Mutex wraps every sess access including the nil check.
- T-04-05 (unauthenticated /metrics): accepted by design (demo project).
- T-04-06 (port conflict DoS): accepted by design (single-instance demo).

## Self-Check: PASSED

- cmd/alice-pq/main.go — FOUND
- cmd/bob-pq/main.go — FOUND
- Commit 724f8cd — verified (alice-pq instrumentation)
- Commit 099745d — verified (bob-pq instrumentation)
- go build ./... — PASSED
- go test ./... — PASSED (22 tests, 11 packages)
