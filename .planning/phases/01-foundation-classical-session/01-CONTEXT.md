# Phase 1: Foundation + Classical Session - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Monorepo scaffolding, shared helper packages (`internal/keys`, `internal/metrics`, `internal/transport`), and a working X3DH + Double Ratchet classical session verified by unit tests. No Centrifugo integration, no PQ code, no Docker. All downstream phases depend on this foundation being clean and building successfully.

</domain>

<decisions>
## Implementation Decisions

### cmd/ Binaries
- **D-01:** All four cmd/ binaries (`alice-classical`, `bob-classical`, `alice-pq`, `bob-pq`) are stub `main()` functions only — `package main` + blank `func main() {}`. Real wiring happens in Phase 3 (Centrifugo integration). Only requirement: `go build ./...` succeeds from repo root.

### internal/transport Test Strategy
- **D-02:** Transport test uses `//go:build integration` guard. The test verifies connect/subscribe/publish/disconnect against a real Centrifugo instance, but is excluded from the default `go test ./...` run. Developer opts in manually; CI can skip without a running Centrifugo. Success criterion #4 is satisfied by the existence and correctness of this test, not by it running in every CI job.

### internal/metrics Histogram Registration
- **D-03:** `internal/metrics` registers ALL final Phase 4 histogram names upfront:
  - `ratchet_message_wire_bytes` (wire size histogram)
  - `ratchet_handshake_duration_seconds` (handshake duration histogram)
  - `ratchet_encrypt_duration_seconds` (encrypt duration histogram)
  - `ratchet_decrypt_duration_seconds` (decrypt duration histogram)
  Phase 4 (OBS-01-03) only calls `.Observe()` — no edits to this package. Prevents cross-phase code churn.

### go-doubleratchet v0.0.2 API
- **D-04:** Session constructor signature: `dr.New(sharedKey, bobDHPublicKey, crypto)` — single constructor for both initiator and responder roles. Alice passes Bob's DH public key; Bob passes Alice's DH public key (or own keypair depending on role).
- **D-05:** `Session.Encrypt(plaintext []byte)` returns `(*dr.Message, error)`.
- **D-06:** `Session.Decrypt(msg *dr.Message)` returns `([]byte, error)`.
- **D-07:** Crypto provider: `dr.DefaultCrypto()` — standard AES/HMAC provider, no custom implementation.
- **D-08:** The library is `github.com/KushnerykPavel/go-doubleratchet v0.0.2` — pinned exact version in `go.mod`. No substitutions.

### Claude's Discretion
- X3DH prekey bundle struct field layout (IK, SPK, SPK_sig presence/absence) — Claude implements minimal bundle sufficient for the CLASS-03 unit test.
- Test helper setup for the integration-tagged transport test — Claude chooses config format.
- Exact histogram bucket boundaries — Claude uses Prometheus defaults unless constrained by Phase 4 requirements.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Library API
- `go.mod` (to be created) — must pin `github.com/KushnerykPavel/go-doubleratchet v0.0.2` exactly
- `github.com/KushnerykPavel/go-doubleratchet` v0.0.2 source — `dr.New`, `Session`, `*dr.Message` types

### Requirements
- `.planning/REQUIREMENTS.md` §Foundation (FOUND-01 through FOUND-04) — exact package layout and helper contracts
- `.planning/REQUIREMENTS.md` §Classical Protocol (CLASS-01 through CLASS-03) — X3DH implementation constraints

### Project Constraints
- `.planning/PROJECT.md` §Constraints — no CGo, no go.work, Go 1.23+, `go-doubleratchet v0.0.2` pinned

No external ADRs. Requirements fully captured in decisions above and REQUIREMENTS.md.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — empty repo. All packages created from scratch.

### Established Patterns
- None yet established. Phase 1 sets the patterns for all downstream phases.

### Integration Points
- `internal/keys` → used by `internal/classical`, `internal/pq`, and all cmd/ binaries
- `internal/metrics` → used by all four cmd/ binaries (Phase 4 calls Observe())
- `internal/transport` → used by all four cmd/ binaries (Phase 3 wires it)
- `internal/classical` → used by `cmd/alice-classical` and `cmd/bob-classical`

</code_context>

<specifics>
## Specific Ideas

- Blog demo context: code must be readable, not optimized — prefer clarity in X3DH implementation over brevity
- `internal/keys.toKey32()` must reject incorrect-length input (exact success criterion #2) — test this edge case explicitly
- Unit test for classical session (CLASS-03): must assert both `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` AND `alice.RootKey == bob.RootKey` after handshake

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-foundation-classical-session*
*Context gathered: 2026-04-27*
