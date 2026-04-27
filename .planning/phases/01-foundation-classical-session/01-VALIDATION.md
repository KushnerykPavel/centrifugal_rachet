---
phase: 1
slug: foundation-classical-session
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-27
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
| 1-01-01 | 01 | 1 | FOUND-01 | — | N/A | build | `go build ./...` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | FOUND-02 | — | Rejects incorrect-length input | unit | `go test ./internal/keys/...` | ❌ W0 | ⬜ pending |
| 1-02-01 | 02 | 1 | FOUND-03 | — | N/A | unit | `go test ./internal/metrics/...` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 1 | FOUND-04 | — | N/A | integration | `go test -tags integration ./internal/transport/...` | ❌ W0 | ⬜ pending |
| 1-04-01 | 04 | 2 | CLASS-01 | — | N/A | unit | `go test ./internal/classical/...` | ❌ W0 | ⬜ pending |
| 1-04-02 | 04 | 2 | CLASS-02 | — | N/A | unit | `go test ./internal/classical/...` | ❌ W0 | ⬜ pending |
| 1-04-03 | 04 | 2 | CLASS-03 | — | N/A | unit | `go test -run TestClassicalSession ./internal/classical/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/keys/keys_test.go` — stubs for FOUND-02
- [ ] `internal/metrics/metrics_test.go` — stubs for FOUND-03
- [ ] `internal/classical/classical_test.go` — stubs for CLASS-01, CLASS-02, CLASS-03
- [ ] `internal/transport/transport_integration_test.go` — build-tagged stub for FOUND-04

*All test files created with failing stubs before implementation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Transport connect/subscribe/publish/disconnect | FOUND-04 | Requires live Centrifugo instance; integration tag guards default run | Start Centrifugo locally, run `go test -tags integration ./internal/transport/...` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
