---
phase: 03-centrifugo-integration
plan: "02"
subsystem: transport
tags: [centrifuge-go, classical, x3dh, double-ratchet, prometheus, websocket]

requires:
  - phase: 03-01
    provides: "protocol.Envelope, RatchetPayload, MarshalEnvelope, channel constants (ChannelClassical)"
  - phase: 02-pq-session
    provides: "internal/classical — NewResponderBundle, InitiatorHandshake, ResponderHandshake, Session.Encrypt/Decrypt"
  - phase: 01
    provides: "internal/transport — Client.Connect/Subscribe/Publish/Disconnect; internal/metrics — Handler()"

provides:
  - "cmd/bob-classical/main.go: publishes prekey_bundle on ch-classical, completes ResponderHandshake on initial_msg, echoes 5 ratchet_msg envelopes with RatchetPayload JSON, exits after 5th echo"
  - "cmd/alice-classical/main.go: subscribes ch-classical, waits 30s for prekey_bundle, performs InitiatorHandshake, sends 5 ratchet_msg envelopes with RatchetPayload JSON, exits after 5 echoes received"

affects:
  - 03-03-pq-pair-binaries
  - 04-metrics-instrumentation
  - 05-docker-compose

tech-stack:
  added: []
  patterns:
    - "go func() wrapping every OnPublication handler body to avoid cbQueue deadlock (D-10/CENT-04)"
    - "sync/atomic.AddInt32 on int32 counters for data-race-free exit signalling"
    - "var sub *centrifuge.Subscription declared before Subscribe; closure captures by reference (RESEARCH.md Open Question 2)"
    - "bundleCh := make(chan []byte, 1) with select+default non-blocking send for duplicate bundle rejection"
    - "RatchetPayload JSON double-layer: outer Envelope.Payload = classical.Message JSON; inner decrypted bytes = RatchetPayload JSON (D-05)"

key-files:
  created:
    - cmd/bob-classical/main.go
    - cmd/alice-classical/main.go
  modified: []

key-decisions:
  - "Import centrifuge-go directly in binaries (not just transport) so var sub *centrifuge.Subscription can be declared before the Subscribe closure, enabling safe capture by reference"

patterns-established:
  - "Pattern: Declare sub before Subscribe closure, assign with = not :=, so the handler goroutine reads sub after Subscribe returns (safe, no handler fires before Subscribe completes)"
  - "Pattern: sess nil-guard before every sess.Decrypt call in handlers — drops message with log.Printf, no panic on out-of-order delivery"

requirements-completed: [CENT-03, CENT-04, CENT-05]

duration: 8min
completed: "2026-04-28"
---

# Phase 03 Plan 02: Classical Pair Binaries Summary

**Classical encrypted-chat pair over Centrifugo ch-classical: Bob publishes X3DH prekey bundle, Alice initiates handshake, both exchange 5 Double Ratchet messages with RatchetPayload JSON and exit cleanly**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-04-28T06:11:00Z
- **Completed:** 2026-04-28T06:19:16Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `cmd/bob-classical/main.go`: generates prekey bundle, publishes on ch-classical, completes X3DH responder handshake, decrypts+re-encrypts 5 ratchet messages as echoes with `RatchetPayload{Seq, Text: "echo: ..."}`, exits via `os.Exit(0)` after 5th echo
- `cmd/alice-classical/main.go`: subscribes ch-classical with 30s timeout for Bob's bundle, performs X3DH initiator handshake, publishes `initial_msg`, sends 5 `ratchet_msg` envelopes each carrying `RatchetPayload{Seq, Text: "hello from alice-classical N"}`, exits after receiving 5 echoes
- All OnPublication handler bodies wrapped in `go func()` (CENT-04 / D-10 cbQueue deadlock prevention)
- Both counters (`echoCount`, `recvCount`) use `sync/atomic.AddInt32` on `int32` (CENT-03 / T-3-02-03 data race prevention)
- `go build ./...` passes with zero errors

## Task Commits

1. **Task 1: Implement cmd/bob-classical/main.go** - `4fe57ff` (feat)
2. **Task 2: Implement cmd/alice-classical/main.go** - `f2ec9e7` (feat)

## Files Created/Modified

- `cmd/bob-classical/main.go` — Bob classical binary: prekey bundle publish, ResponderHandshake, echo loop with atomic counter and RatchetPayload JSON
- `cmd/alice-classical/main.go` — Alice classical binary: 30s bundle wait, InitiatorHandshake, 5-message send with RatchetPayload JSON, echo receive with atomic counter

## Decisions Made

- Imported `centrifuge-go` directly in binaries (in addition to `internal/transport`) to type `var sub *centrifuge.Subscription` before the Subscribe call. This is the only safe pattern when the closure must reference `sub` for Publish calls — the plan's RESEARCH.md explicitly calls this out as Open Question 2, and the pattern requires the variable be visible to the closure before Subscribe is called.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed undefined `sub` in closure by declaring var before Subscribe**
- **Found during:** Task 1 (bob-classical implementation)
- **Issue:** Using `:=` shorthand for Subscribe return value created a new local `sub` variable, making it invisible to the closure goroutine, resulting in `undefined: sub` compile error
- **Fix:** Added `var sub *centrifuge.Subscription` declaration before the Subscribe call; assigned with plain `=`; added `centrifuge-go` import directly in the binary for the type declaration
- **Files modified:** `cmd/bob-classical/main.go`, `cmd/alice-classical/main.go`
- **Verification:** `go build ./cmd/bob-classical/... ./cmd/alice-classical/...` exits 0
- **Committed in:** `4fe57ff`, `f2ec9e7` (part of each task commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — compile bug)
**Impact on plan:** Required for compilation. Plan mentioned this pattern in RESEARCH.md but the action block used `:=` which would not compile. No scope creep.

## Issues Encountered

The plan's action block showed `sub, err := cl.Subscribe(...)` with `:=` but simultaneously required the closure to reference `sub` for `cl.Publish(context.Background(), sub, raw)`. In Go, `:=` creates a new variable in the current scope, so the closure captures the unassigned outer variable only if it is declared with `var` before the closure. Switched to `var sub *centrifuge.Subscription` + plain `=` assignment, which is exactly what RESEARCH.md Open Question 2 documents.

## Known Stubs

None — both binaries are fully wired: crypto, transport, and message framing all operational.

## Next Phase Readiness

- Both classical binaries compile and follow all mandatory protocol patterns (CENT-03, CENT-04, CENT-05)
- Plan 03-03 can now implement the PQ pair binaries following the identical pattern
- Phase 04 metrics instrumentation can instrument `sess.Encrypt`/`sess.Decrypt` call sites in these files

---
*Phase: 03-centrifugo-integration*
*Completed: 2026-04-28*
