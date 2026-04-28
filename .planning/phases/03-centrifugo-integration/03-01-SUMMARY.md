---
phase: 03-centrifugo-integration
plan: 01
subsystem: protocol
tags: [go, json, centrifugo, envelope, ratchet, protocol]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: go.mod, module path github.com/KushnerykPavel/centrifugal-ratchet
  - phase: 02-pq-session
    provides: internal/pq and internal/classical packages that Wave 2 binaries will use alongside protocol
provides:
  - internal/protocol/envelope.go — Envelope, RatchetPayload, TypeX constants, ChannelX constants, MarshalEnvelope
  - centrifugo/config.json — Centrifugo 5.x insecure-mode config for local development
affects:
  - 03-02 (bob-classical, alice-classical binaries — import internal/protocol)
  - 03-03 (bob-pq, alice-pq binaries — import internal/protocol)
  - 05-docker-compose (mounts centrifugo/ directory)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Envelope pattern: json.RawMessage Payload with Type string for channel dispatch"
    - "External test package: package protocol_test isolates test imports"
    - "MarshalEnvelope helper: single function for consistent wire encoding across all senders"

key-files:
  created:
    - internal/protocol/envelope.go
    - internal/protocol/envelope_test.go
    - centrifugo/config.json
  modified: []

key-decisions:
  - "Envelope.Payload is json.RawMessage (not interface{}) — defers inner-type parsing to receiver, enables type-switch dispatch pattern"
  - "centrifugo/config.json uses nested client.insecure (Centrifugo 5.x schema) not deprecated root-level insecure"
  - "Channel constants locked: ch-classical and ch-pq — all four binaries must use these values"

patterns-established:
  - "TestSubject_Condition naming convention for all protocol tests"
  - "No third-party assertion libraries — stdlib testing only in internal/protocol"

requirements-completed:
  - CENT-01
  - CENT-02
  - CENT-05

# Metrics
duration: 2min
completed: 2026-04-28
---

# Phase 3 Plan 01: Protocol Package and Centrifugo Config Summary

**Shared wire format package (Envelope, RatchetPayload, type/channel constants, MarshalEnvelope) and Centrifugo 5.x insecure-mode config enabling all four Wave 2 binaries to compile**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-28T06:13:01Z
- **Completed:** 2026-04-28T06:15:04Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Created `internal/protocol` package with all types, constants, and helper locked by design decisions D-01, D-02, D-05, D-11
- Added 5 unit tests covering round-trip marshal/unmarshal, all three type constants, channel constant values, error path, and RatchetPayload JSON structure
- Created `centrifugo/config.json` with Centrifugo 5.x nested `client.insecure` schema for insecure local development mode

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/protocol/envelope.go** - `d431bfe` (feat)
2. **Task 2: Create internal/protocol/envelope_test.go** - `6029b2b` (test)
3. **Task 3: Create centrifugo/config.json** - `6036529` (chore)

## Files Created/Modified

- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/protocol/envelope.go` — Envelope struct, RatchetPayload struct, TypePrekeyBundle/TypeInitialMsg/TypeRatchetMsg constants, ChannelClassical/ChannelPQ constants, MarshalEnvelope helper
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/internal/protocol/envelope_test.go` — 5 unit tests using stdlib testing only, external test package protocol_test
- `/Users/pavelkushneryk/Documents/vsprojects/linkedin_blog/centrifugal_rachet/centrifugo/config.json` — Centrifugo 5.x config with client.insecure=true, health=true, port=8000

## Decisions Made

- Followed locked decisions D-01 (Envelope struct), D-02 (type constants), D-05 (RatchetPayload), D-11 (channel constants) exactly as specified
- Used external test package `protocol_test` matching project convention from metrics_test.go
- centrifugo/config.json uses nested `client.insecure` (not deprecated root-level) per RESEARCH.md Centrifugo 5.x schema findings

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Threat Surface Scan

The centrifugo/config.json sets `client.insecure: true`, which disables client authentication. This is documented as T-3-01-03 (accepted risk) in the plan's threat model — explicit blog-demo decision D-07 with no PII and no production users. No new unplanned threat surface introduced.

## Next Phase Readiness

- `internal/protocol` package is ready for import by all four Wave 2 binaries (bob-classical, alice-classical, bob-pq, alice-pq)
- `centrifugo/config.json` is ready to be mounted by Docker Compose in Phase 5 at `./centrifugo:/centrifugo`
- `go build ./...` and `go test ./internal/protocol/...` both exit 0

---
*Phase: 03-centrifugo-integration*
*Completed: 2026-04-28*
