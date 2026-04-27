# Phase 3: Centrifugo Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-27
**Phase:** 03-centrifugo-integration
**Areas discussed:** Message envelope design, Alice/Bob exchange count, Centrifugo config, Alice retry / 30s wait

---

## Message Envelope Design

| Option | Description | Selected |
|--------|-------------|----------|
| internal/protocol package | Shared package, all 4 binaries import it | ✓ |
| Each binary defines its own | Duplicates struct 4x | |

| Option | Description | Selected |
|--------|-------------|----------|
| json.RawMessage payload | Envelope{Type, Payload json.RawMessage}, type-switch dispatch | ✓ |
| Typed union struct | Multiple optional pointer fields | |

**Notes:** json.RawMessage is standard pattern for type-dispatched envelopes.

---

## Alice/Bob Exchange Count

| Option | Description | Selected |
|--------|-------------|----------|
| 5 messages then exit | 10 total per pair, enough for Grafana trend | ✓ |
| 3 messages then exit | Fewer samples | |
| Continuous loop until Ctrl-C | Indefinite, harder to test | |

**Notes:** Alice sends 5, Bob echoes 5. Both exit after last message.

---

## Centrifugo Config

| Option | Description | Selected |
|--------|-------------|----------|
| JSON (config.json) | Explicit for blog readers | ✓ |
| YAML (config.yaml) | More concise | |

| Option | Description | Selected |
|--------|-------------|----------|
| centrifugo/config.json | Dedicated directory, Docker volume mount | ✓ |
| configs/centrifugo.json | Grouped with future configs | |

---

## Alice Retry / 30s Wait

| Option | Description | Selected |
|--------|-------------|----------|
| Channel + select with 30s deadline | Idiomatic Go, testable | ✓ |
| Polling loop with time.Sleep | Less idiomatic | |

| Option | Description | Selected |
|--------|-------------|----------|
| log.Fatalf and os.Exit(1) | Clear Docker log output | ✓ |
| Retry indefinitely | Hard to debug failures | |

---

## Claude's Discretion

- Error handling style within binaries
- Integration test coverage (Phase 5 docker compose)
- internal/protocol helper functions beyond envelope.go

## Deferred Ideas

None.
