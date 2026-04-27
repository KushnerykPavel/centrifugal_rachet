---
phase: 2
slug: pq-session
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-27
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none (standard `go test`) |
| **Quick run command** | `go test ./internal/pq/... -run TestPQ -v` |
| **Full suite command** | `go test ./... -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pq/... -v`
- **After every plan wave:** Run `go test ./... -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 0 | PQ-01, PQ-02, PQ-03, PQ-04 | T-2-04 | Test stubs compile | unit | `go build ./internal/pq/...` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | PQ-02 | T-2-04 | Snapshot deep-copy: mutate original, snapshot unchanged | unit | `go test ./internal/pq/... -run TestMLKEMProviderSnapshot -v` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | PQ-02 | T-2-03 | Close() zeros key material | unit | `go test ./internal/pq/... -run TestMLKEMProviderClose -v` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 2 | PQ-01 | T-2-02 | PQXDH RootKey matches on both sides | unit | `go test ./internal/pq/... -run TestPQXDHHandshake -v` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 2 | PQ-03 | — | bob.Decrypt(alice.Encrypt(plaintext)) == plaintext | unit | `go test ./internal/pq/... -run TestPQSession -v` | ❌ W0 | ⬜ pending |
| 02-03-03 | 03 | 2 | PQ-04 | — | len(pqJSON) > len(classicalJSON) for same plaintext | unit | `go test ./internal/pq/... -run TestPQWireOverhead -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pq/pq.go` — package scaffold (Session struct, PrekeyBundle, InitialMessage types)
- [ ] `internal/pq/provider.go` — MLKEMProvider scaffold (struct + method stubs)
- [ ] `internal/pq/pq_test.go` — test file with PQ-01 through PQ-04 test stubs (build-only initially)

*Wave 0 creates compilable skeletons so `go build ./...` passes immediately.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| MLKEMProvider epoch sequencing correctness | PQ-03 | First integration run validates whether outputKey timing in Send()/Receive() is correct | Run PQ-03 test and verify no epoch mismatch panic in SPQR layer |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
