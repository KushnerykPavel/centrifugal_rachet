---
phase: 1
slug: foundation-classical-session
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-27
audited: 2026-04-27
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — standard Go test files |
| **Quick run command** | `go test ./internal/...` |
| **Full suite command** | `go test ./... && go build ./...` |
| **Integration tests** | `go test -tags integration ./internal/transport/...` (requires live Centrifugo) |
| **Estimated runtime** | ~5 seconds (unit), ~15 seconds (integration) |

---

## Sampling Rate

- **After every task commit:** Run `go build ./...`
- **After every plan wave:** Run `go test ./internal/...`
- **Before `/gsd-verify-work`:** Full suite must be green (`go test ./... && go build ./...`)
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | FOUND-01 | — | N/A | build | `go build ./...` | ✅ | ✅ green |
| 1-01-02 | 01 | 1 | FOUND-02 | — | Rejects incorrect-length input | unit | `go test ./internal/keys/...` | ✅ internal/keys/keys_test.go | ✅ green |
| 1-02-01 | 02 | 2 | FOUND-03 | — | N/A | unit | `go test ./internal/metrics/...` | ✅ internal/metrics/metrics_test.go | ✅ green |
| 1-03-01 | 03 | 2 | FOUND-04 | — | N/A | integration | `go test -tags integration ./internal/transport/...` | ✅ internal/transport/transport_integration_test.go | manual-only |
| 1-04-01 | 04 | 3 | CLASS-01 | — | N/A | unit | `go test ./internal/classical/...` | ✅ internal/classical/classical_test.go | ✅ green |
| 1-04-02 | 04 | 3 | CLASS-02 | — | N/A | unit | `go test ./internal/classical/...` | ✅ internal/classical/classical_test.go | ✅ green |
| 1-04-03 | 04 | 3 | CLASS-03 | — | N/A | unit | `go test -run TestClassicalSession ./internal/classical/...` | ✅ internal/classical/classical_test.go | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All Wave 0 test files created during TDD execution:

- [x] `internal/keys/keys_test.go` — 5 tests for FOUND-02 (TestToKey32_ValidInput, TooShort, TooLong, Empty, NoCopy)
- [x] `internal/metrics/metrics_test.go` — 3 tests for FOUND-03 (Status200, ContainsHistograms, CustomRegistry)
- [x] `internal/classical/classical_test.go` — 3 tests for CLASS-01/02/03 (TestClassicalSession, BidirectionalExchange, ErrorOnNilMsg)
- [x] `internal/transport/transport_integration_test.go` — build-tagged integration test for FOUND-04

*All test files written before implementation (TDD RED→GREEN cycle confirmed in SUMMARY files).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Transport connect/subscribe/publish/disconnect | FOUND-04 | Requires live Centrifugo instance; integration tag guards default run | Start Centrifugo locally, run `go test -tags integration ./internal/transport/...` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-27

---

## Validation Audit 2026-04-27

| Metric | Count |
|--------|-------|
| Requirements audited | 7 |
| Gaps found | 0 |
| COVERED (automated) | 6 |
| MANUAL-ONLY | 1 (FOUND-04 — integration build tag, by design D-02) |
| Resolved | 0 needed |
| Escalated | 0 |

All test files verified present on filesystem. All unit tests pass (`go test ./internal/... -race`). Build passes (`go build ./...`). FOUND-04 transport integration test excluded from default run per CONTEXT.md D-02 decision — this is intentional, not a gap.
