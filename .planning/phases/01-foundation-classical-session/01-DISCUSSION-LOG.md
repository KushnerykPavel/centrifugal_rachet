# Phase 1: Foundation + Classical Session - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-27
**Phase:** 01-foundation-classical-session
**Areas discussed:** cmd/ binary scope, transport test coverage, Metrics histogram scope, go-doubleratchet v0.0.2 API

---

## cmd/ Binary Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Stub main() only | Each binary is package main + blank main(). Builds cleanly. Real wiring in Phase 3. | ✓ |
| Minimal runnable scaffold | Each binary imports internal/ package, instantiates session, prints a line. | |
| You decide | Claude picks whatever satisfies go build ./... cleanly. | |

**User's choice:** Stub main() only
**Notes:** Real wiring deferred to Phase 3 (Centrifugo integration).

---

## Transport Test Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Compile + interface check only | Unit test verifies method signatures. No live Centrifugo. | |
| Live Centrifugo required | Success criterion implies real connection. Centrifugo in test helper or Docker. | |
| Build tag guard | Real connection test exists but gated with //go:build integration. CI can skip. | ✓ |

**User's choice:** Build tag guard + manual
**Notes:** Developer opts in with `go test -tags integration ./...`; CI skips by default.

---

## Metrics Histogram Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Register all final names now | internal/metrics defines all 4 final histogram names upfront. Phase 4 just calls Observe(). | ✓ |
| Placeholder structure only | Registry + handler but no specific histograms. Phase 4 registers real names. | |
| You decide | Claude picks approach minimizing cross-phase editing. | |

**User's choice:** Register all final names now
**Notes:** Avoids cross-phase code edits in Phase 4.

---

## go-doubleratchet v0.0.2 API

| Option | Description | Selected |
|--------|-------------|----------|
| dr.New(sharedKey, bobDHPub, crypto) | Single constructor. Alice passes Bob's key, Bob passes Alice's key. | ✓ |
| dr.NewWithRemoteKey / dr.NewWithSharedKey | Two constructors for initiator vs responder. | |
| I'll describe it | Signatures don't match these patterns. | |

**Session.Encrypt return:** `*dr.Message, error` (pointer, not value)
**Session.Decrypt accept:** `*dr.Message`
**Crypto provider:** `dr.DefaultCrypto()` — standard AES/HMAC, no custom impl

**User's choice:** dr.New(sharedKey, bobDHPub, crypto) with DefaultCrypto()
**Notes:** Library is author's own fork — API confirmed by author.

---

## Claude's Discretion

- X3DH prekey bundle struct field layout
- Test helper configuration for integration-tagged transport test
- Histogram bucket boundaries (Prometheus defaults)

## Deferred Ideas

None.
