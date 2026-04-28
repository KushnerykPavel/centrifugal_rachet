---
phase: 06-readme-blog-content
plan: "03"
subsystem: documentation
tags: [readme, blog, verification, api-validation, byte-sizes]
dependency_graph:
  requires: [06-01, 06-02]
  provides: [README.md (verified), blog-draft.md (verified)]
  affects: []
tech_stack:
  added: []
  patterns: [source-truth-validation, footnote-accuracy]
key_files:
  created: []
  modified:
    - README.md
    - blog-draft.md
decisions:
  - "Byte numbers (~120/~1208/~40/~1128) cannot be confirmed without a live docker compose run — updated footnotes to accurately state 'planning-phase estimates' with TODO to verify"
  - "ML-KEM-768 key sizes (1184/1088 bytes) confirmed HIGH confidence by unit-test assertions in pq_test.go (TestMLKEMProviderKEMProtocol and TestMLKEMProviderSnapshot)"
  - "All four API function names confirmed correct by grep against source files — no blog corrections needed"
  - "aliceSCKA not present in primary snippets (Pitfall 3 PASSED)"
  - "Close() asymmetry correctly documented in prose only — no code snippet correction needed (Pitfall 4 PASSED)"
metrics:
  duration: 4min
  completed: "2026-04-28T13:14:26Z"
  tasks_completed: 1
  files_changed: 2
---

# Phase 06 Plan 03: Verification Pass Summary

API validation pass confirming all blog-draft.md function names exist in source, and correcting both README.md and blog-draft.md comparison table footnotes from false "measured from live run" claims to accurate "planning-phase estimates with TODO to verify."

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | API validation + byte number verification + footnote correction | ee0b7a1 | README.md, blog-draft.md |

## Byte Number Verification Status

Docker was not running during this verification pass. Source code and test files were checked instead:

| Metric | Value | Confidence | Source |
|--------|-------|------------|--------|
| ML-KEM-768 encap key | 1184 bytes | HIGH | Unit test assertion: `require.Equal(t, 1184, len(msg))` in `pq_test.go` line 65 |
| ML-KEM-768 ciphertext | 1088 bytes | HIGH | Unit test assertion: `require.Equal(t, 1088, len(bobMsg2))` in `pq_test.go` line 106 |
| Classical handshake initial msg | ~120 bytes | LOW (estimate) | REQUIREMENTS.md planning estimate — not confirmed by test or live run |
| PQ handshake initial msg | ~1208 bytes | LOW (estimate) | REQUIREMENTS.md planning estimate — not confirmed by test or live run |
| Classical ratchet step | ~40 bytes | LOW (estimate) | REQUIREMENTS.md planning estimate — not confirmed by test or live run |
| PQ ratchet step | ~1128 bytes | LOW (estimate) | REQUIREMENTS.md planning estimate — not confirmed by test or live run |

TODO: Run `docker compose up` and verify full message wire sizes via Grafana `ratchet_message_wire_bytes` histogram at http://localhost:3000.

## API Validation Results

All four D-08 code snippet function names confirmed against source files:

| Function | Source File | Status |
|----------|-------------|--------|
| `classical.NewResponderBundle()` | internal/classical/classical.go:52 | CONFIRMED |
| `classical.InitiatorHandshake(bundle)` | internal/classical/classical.go:74 | CONFIRMED |
| `classical.ResponderHandshake(priv, initMsg)` | internal/classical/classical.go:108 | CONFIRMED |
| `classical.Session.Encrypt(plaintext)` | internal/classical/classical.go:134 | CONFIRMED |
| `classical.Session.Decrypt(msg)` | internal/classical/classical.go:144 | CONFIRMED |
| `classical.Session.Close()` — no error return | internal/classical/classical.go:156 | CONFIRMED |
| `pq.NewResponderBundle()` | internal/pq/pq.go:40 | CONFIRMED |
| `pq.InitiatorHandshake(bundle)` | internal/pq/pq.go:85 | CONFIRMED |
| `pq.ResponderHandshake(priv, initMsg)` | internal/pq/pq.go:112 | CONFIRMED |
| `pq.Session.Encrypt(plaintext)` | internal/pq/pq.go:138 | CONFIRMED |
| `pq.Session.Decrypt(msg)` | internal/pq/pq.go:149 | CONFIRMED |
| `pq.Session.Close() error` — returns error | internal/pq/pq.go:161 | CONFIRMED |
| `ratchet_message_wire_bytes` (metric name) | internal/metrics/metrics.go:27 | CONFIRMED |
| `ratchet_handshake_duration_seconds` (metric name) | internal/metrics/metrics.go:32 | CONFIRMED |
| `prometheus.NewRegistry()` | internal/metrics/metrics.go:24 | CONFIRMED |
| `Reg.MustRegister(...)` | internal/metrics/metrics.go:47 | CONFIRMED |

### Pitfall Checks

**Pitfall 3 (aliceSCKA not in primary snippet):** PASSED — `grep aliceSCKA blog-draft.md` returns 0 matches. The `aliceSCKA := &MLKEMProvider{}` line lives inside `InitiatorHandshake` in `pq.go`; blog correctly notes it is "under the hood."

**Pitfall 4 (Close() asymmetry documented):** PASSED — blog-draft.md line 187 correctly states: `classical.Session.Close()` returns nothing; `pq.Session.Close()` returns `error`. No code snippets use `Close()` so no call-site correction was needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected false "measured from live run" claim in README.md**
- **Found during:** Task 1 verification — Docker not running; checkpoint_note confirmed docker should not be started
- **Issue:** README.md footnote said `* measured from a live docker compose up run` — this was written optimistically in plan 06-01; the numbers are REQUIREMENTS.md planning estimates that have never been confirmed by a live run
- **Fix:** Changed footnote symbol from `*` to `†`, replaced claim with accurate "planning-phase estimates (REQUIREMENTS.md); not yet confirmed from a live run — TODO: run docker compose up"
- **Files modified:** README.md
- **Commit:** ee0b7a1

**2. [Rule 1 - Bug] Corrected false "measured from live run" claim in blog-draft.md**
- **Found during:** Task 1 — same root cause as above; blog-draft.md wire-size comparison table had identical false footnote
- **Fix:** Updated footnote to "planning-phase estimates; verify by running docker compose up"
- **Files modified:** blog-draft.md
- **Commit:** ee0b7a1

## Known Stubs

The wire-size numbers (~120/~1208/~40/~1128) are LOW confidence estimates. Both README.md and blog-draft.md now accurately label them as "planning-phase estimates" with an explicit TODO. The ML-KEM-768 component sizes (1184/1088) are HIGH confidence per test assertions. Full message sizes require a live stack run.

## Threat Flags

None. This plan only modifies documentation files — no new network endpoints, auth paths, or schema changes.

## Self-Check

- [x] README.md updated: `grep "planning-phase estimates" README.md` matches (line 56)
- [x] blog-draft.md updated: `grep "planning-phase estimates" blog-draft.md` matches (line 202)
- [x] `grep -c "InitiatorHandshake\|ResponderHandshake" blog-draft.md` = 9 (2+ classical + PQ snippets confirmed)
- [x] `grep "aliceSCKA" blog-draft.md` = 0 matches
- [x] Commit ee0b7a1 exists

## Self-Check: PASSED
