---
phase: 03-centrifugo-integration
plan: "03"
subsystem: transport
tags: [centrifuge-go, pq, pqxdh, triple-ratchet, prometheus, websocket, mlkem768]

requires:
  - phase: 03-01
    provides: "protocol.Envelope, RatchetPayload, MarshalEnvelope, ChannelPQ constant"
  - phase: 03-02
    provides: "Established binary structure pattern: var sub before Subscribe, go func() handler, atomic counters"
  - phase: 02-pq-session
    provides: "internal/pq — NewResponderBundle, InitiatorHandshake, ResponderHandshake, Session.Encrypt/Decrypt/Close"
  - phase: 01
    provides: "internal/transport — Client.Connect/Subscribe/Publish/Disconnect; internal/metrics — Handler()"

provides:
  - "cmd/bob-pq/main.go: generates PQXDH prekey bundle, publishes on ch-pq, completes PQXDH ResponderHandshake on initial_msg, echoes 5 TripleRatchetMessage envelopes with RatchetPayload JSON, exits after 5th echo"
  - "cmd/alice-pq/main.go: subscribes ch-pq, waits 30s for prekey_bundle, unmarshal into pqxdh.PrekeyBundle, performs PQXDH InitiatorHandshake, sends 5 ratchet_msg envelopes with RatchetPayload JSON, exits after 5 echoes received"

affects:
  - 04-metrics-instrumentation
  - 05-docker-compose

tech-stack:
  added: []
  patterns:
    - "pqxdh.PrekeyBundle (library type) as unmarshal target for Alice — not a custom struct (Pitfall 6)"
    - "go func() wrapping every OnPublication handler body (D-10/CENT-04/T-3-03-01)"
    - "sync/atomic.AddInt32 on int32 counters for data-race-free exit signalling (T-3-03-03)"
    - "var sub *centrifuge.Subscription declared before Subscribe; closure captures by reference"
    - "bundleCh := make(chan []byte, 1) with select+default non-blocking send for duplicate bundle rejection (T-3-03-05)"
    - "RatchetPayload JSON double-layer: outer Envelope.Payload = TripleRatchetMessage JSON; inner decrypted bytes = RatchetPayload JSON (D-05)"
    - "sess nil-guard before sess.Decrypt in TypeRatchetMsg handler (T-3-03-02)"
    - "pq.Session.Close() returns error (unlike classical.Session.Close) — deferred via os.Exit; not explicitly deferred to avoid goroutine interaction"

key-files:
  created:
    - cmd/bob-pq/main.go
    - cmd/alice-pq/main.go
  modified: []

key-decisions:
  - "Applied same var-sub-before-Subscribe pattern established in plan 03-02 to avoid undefined sub in closure"
  - "Imported centrifuge-go directly for *centrifuge.Subscription type declaration — identical to classical pair"
  - "pqxdh imported directly in alice-pq for pqxdh.PrekeyBundle unmarshal target type (required by Pitfall 6)"
  - "bob-pq does not need explicit pqxdh import — bundle flows through pq facade into MarshalEnvelope as any"

requirements-completed: [CENT-03, CENT-04, CENT-05]

duration: 4min
completed: "2026-04-28"
---

# Phase 03 Plan 03: PQ Pair Binaries Summary

**PQXDH + Triple Ratchet encrypted-chat pair over Centrifugo ch-pq: Bob generates and publishes prekey bundle, Alice performs PQXDH initiator handshake using pqxdh.PrekeyBundle unmarshal target, both exchange 5 TripleRatchetMessage envelopes with RatchetPayload JSON and exit cleanly**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-04-28T06:20:30Z
- **Completed:** 2026-04-28T06:23:05Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `cmd/bob-pq/main.go`: generates PQXDH prekey bundle via `pq.NewResponderBundle()`, publishes on `ch-pq`, completes `pq.ResponderHandshake` on `initial_msg`, decrypts TripleRatchetMessage + unmarshals `RatchetPayload`, echoes `RatchetPayload{Seq, Text: "echo: ..."}` encrypted, exits via `os.Exit(0)` after 5th echo
- `cmd/alice-pq/main.go`: subscribes `ch-pq` with 30s timeout for Bob's bundle, unmarshals directly into `pqxdh.PrekeyBundle` (Pitfall 6), performs `pq.InitiatorHandshake`, publishes `initial_msg`, sends 5 `ratchet_msg` envelopes each carrying `RatchetPayload{Seq, Text: "hello from alice-pq N"}` as encrypted JSON, exits after receiving 5 echoes
- All OnPublication handler bodies wrapped in `go func()` (CENT-04 / D-10 / T-3-03-01)
- Both counters (`echoCount`, `recvCount`) use `sync/atomic.AddInt32` on `int32` (CENT-03 / T-3-03-03)
- `go build ./...` passes; `go test ./...` passes (22 tests, 11 packages)
- Channel isolation verified: `ChannelClassical` absent from both PQ binaries (T-3-03-06)

## Task Commits

1. **Task 1: Implement cmd/bob-pq/main.go** - `f7b02ae` (feat)
2. **Task 2: Implement cmd/alice-pq/main.go** - `fa725a6` (feat)

## Files Created/Modified

- `cmd/bob-pq/main.go` — Bob PQ binary: PQXDH bundle publish, ResponderHandshake, TripleRatchetMessage echo loop with atomic counter and RatchetPayload JSON (137 lines added)
- `cmd/alice-pq/main.go` — Alice PQ binary: 30s bundle wait using pqxdh.PrekeyBundle, InitiatorHandshake, 5-message send with RatchetPayload JSON, echo receive with atomic counter (153 lines added)

## Decisions Made

- Applied the `var sub *centrifuge.Subscription` + plain `=` assignment pattern from plan 03-02. The closure references `sub` for `cl.Publish`, which requires the variable to be declared in outer scope before Subscribe is called.
- bob-pq does not need an explicit `pqxdh` import because `*pqxdh.PrekeyBundle` flows from `pq.NewResponderBundle()` directly into `protocol.MarshalEnvelope(..., bundle)` as `any` — the compiler does not require the import.
- alice-pq imports `github.com/KushnerykPavel/go-doubleratchet/pqxdh` explicitly because `var bundle pqxdh.PrekeyBundle` requires the type to be visible at the call site (Pitfall 6).

## Deviations from Plan

None — plan executed exactly as written. The `var sub *centrifuge.Subscription` pattern was already specified in the plan's import/action blocks based on lessons from plan 03-02.

## Known Stubs

None — both PQ binaries are fully wired: PQXDH key exchange, Triple Ratchet encrypt/decrypt, Centrifugo transport, and RatchetPayload JSON framing all operational.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced beyond what the threat model already covers. All seven threats in the plan's STRIDE register are mitigated as specified.

## Self-Check: PASSED

- `cmd/bob-pq/main.go` exists: FOUND
- `cmd/alice-pq/main.go` exists: FOUND
- Commit `f7b02ae` exists: FOUND
- Commit `fa725a6` exists: FOUND
- `go build ./...`: SUCCESS
- `go test ./...`: 22 PASSED

## Next Phase Readiness

- All four binaries (bob-classical, alice-classical, bob-pq, alice-pq) compile and follow all mandatory protocol patterns (CENT-03, CENT-04, CENT-05)
- Phase 04 metrics instrumentation can instrument `sess.Encrypt`/`sess.Decrypt` call sites in all four binary files
- Phase 05 docker-compose can wire up CENTRIFUGO_URL env vars and service definitions for all four binaries

---
*Phase: 03-centrifugo-integration*
*Completed: 2026-04-28*
